package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	iddomain "backend/internal/modules/identity/domain"
	iddto "backend/internal/modules/identity/dto"
	"backend/internal/modules/identity/port"
	"backend/internal/platform/kit"
	"backend/pkg/response"
	"context"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const inviteTTL = 7 * 24 * time.Hour

// invites creates and re-sends team invitations (used by the company service)
type invites struct {
	invitations port.InvitationRepository
	memberships output.MembershipRepository
	branches    output.BranchRepository
	company     output.CompanyRepository
	users       output.UserRepository
	mailer      output.Mailer
	frontendURL string
	now         func() time.Time
}

func (iv invites) acceptURL(raw string) string { return iv.frontendURL + "/invite/" + raw }

// branchRefs names the invitation's branches (deleted ones are left out)
func (iv invites) branchRefs(ctx context.Context, ids []uint) []dto.BranchRef {
	refs := []dto.BranchRef{}
	for _, id := range ids {
		if b, err := iv.branches.GetByID(ctx, id); err == nil {
			refs = append(refs, dto.BranchRef{ID: b.ID, Name: b.Name})
		}
	}
	return refs
}

func (iv invites) toDTO(ctx context.Context, inv iddomain.Invitation) iddto.InvitationDTO {
	return iddto.InvitationDTO{
		ID: inv.ID, Email: inv.Email, Role: inv.Role, Branches: iv.branchRefs(ctx, inv.BranchIDs), ExpiresAt: inv.ExpiresAt,
	}
}

// where says in the email where the person will work
func (iv invites) where(ctx context.Context, inv iddomain.Invitation) string {
	if inv.Role == domain.CompanyAdmin {
		return "every branch"
	}
	names := []string{}
	for _, r := range iv.branchRefs(ctx, inv.BranchIDs) {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}

func (iv invites) send(ctx context.Context, inv iddomain.Invitation, actor *domain.User, raw string) error {
	c, err := iv.company.Get(ctx)
	if err != nil {
		return err
	}
	if err := iv.mailer.SendInvitation(inv.Email, c.Name, actor.Name, inv.Role, iv.where(ctx, inv), iv.acceptURL(raw)); err != nil {
		log.Printf("❌ invitation email to %s failed: %v", inv.Email, err)
		return response.NewError(http.StatusBadGateway, "EMAIL_SEND_FAILED")
	}
	return nil
}

// create stores an invitation and emails the link; if the email cannot be sent the invitation
// is removed again. role and branchIDs are already checked by the caller.
func (iv invites) create(ctx context.Context, actor *domain.User, email, role string, branchIDs []uint) (*iddto.InvitationDTO, error) {
	email = kit.NormalizeEmail(email)
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return nil, badRequest("INVALID_EMAIL")
	}
	if existing, err := iv.users.FindByEmail(ctx, email); err == nil {
		if existing.RunsCompany() {
			return nil, conflict("ALREADY_IN_THE_TEAM")
		}
		works, err := iv.memberships.ListByUser(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		if len(works) > 0 {
			return nil, conflict("ALREADY_IN_THE_TEAM")
		}
	} else if !isNotFound(err) {
		return nil, err
	}
	pending, err := iv.invitations.PendingForEmail(ctx, email, iv.now())
	if err != nil {
		return nil, err
	}
	if pending != nil {
		return nil, conflict("INVITATION_ALREADY_PENDING")
	}

	raw, hash, err := newToken()
	if err != nil {
		return nil, err
	}
	inv := iddomain.Invitation{
		Email: email, Role: role, BranchIDs: branchIDs, TokenHash: hash,
		ExpiresAt: iv.now().Add(inviteTTL), InvitedByID: actor.ID,
	}
	if err := iv.invitations.Create(ctx, &inv); err != nil {
		return nil, err
	}
	if err := iv.send(ctx, inv, actor, raw); err != nil {
		_ = iv.invitations.Delete(ctx, inv.ID)
		return nil, err
	}
	d := iv.toDTO(ctx, inv)
	return &d, nil
}

// resend issues a fresh link (the old one stops working) and a new 7-day expiry
func (iv invites) resend(ctx context.Context, actor *domain.User, inv *iddomain.Invitation) (*iddto.InvitationDTO, error) {
	if inv.AcceptedAt != nil {
		return nil, conflict("INVITATION_ALREADY_ACCEPTED")
	}
	raw, hash, err := newToken()
	if err != nil {
		return nil, err
	}
	inv.TokenHash, inv.ExpiresAt = hash, iv.now().Add(inviteTTL)
	if err := iv.invitations.Update(ctx, inv); err != nil {
		return nil, err
	}
	if err := iv.send(ctx, *inv, actor, raw); err != nil {
		return nil, err
	}
	d := iv.toDTO(ctx, *inv)
	return &d, nil
}

// ---------- accepting ----------

type InvitationServiceImpl struct {
	invitations port.InvitationRepository
	memberships output.MembershipRepository
	branches    output.BranchRepository
	company     output.CompanyRepository
	users       output.UserRepository
	iv          invites
	now         func() time.Time
}

func NewInvitationService(
	invitations port.InvitationRepository,
	memberships output.MembershipRepository,
	branches output.BranchRepository,
	company output.CompanyRepository,
	users output.UserRepository,
	mailer output.Mailer,
	frontendURL string,
) port.InvitationService {
	return &InvitationServiceImpl{
		invitations: invitations, memberships: memberships, branches: branches, company: company, users: users, now: time.Now,
		iv: invites{
			invitations: invitations, memberships: memberships, branches: branches, company: company, users: users,
			mailer: mailer, frontendURL: strings.TrimRight(frontendURL, "/"), now: time.Now,
		},
	}
}

// open finds a usable invitation; used, expired and unknown links look the same
func (s *InvitationServiceImpl) open(ctx context.Context, token string) (*iddomain.Invitation, error) {
	inv, err := s.invitations.GetByTokenHash(ctx, hashToken(token))
	if err != nil {
		return nil, err
	}
	if inv == nil || inv.AcceptedAt != nil || !s.now().Before(inv.ExpiresAt) {
		return nil, notFound("INVITATION_NOT_FOUND_OR_EXPIRED")
	}
	return inv, nil
}

// openBranches keeps the invitation's branches that still exist and are open
func (s *InvitationServiceImpl) openBranches(ctx context.Context, ids []uint) ([]domain.Branch, error) {
	out := []domain.Branch{}
	for _, id := range ids {
		b, err := s.branches.GetByID(ctx, id)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return nil, err
		}
		if !b.Closed {
			out = append(out, *b)
		}
	}
	return out, nil
}

func (s *InvitationServiceImpl) GetInvitation(ctx context.Context, token string) (*iddto.InvitationInfoDTO, error) {
	inv, err := s.open(ctx, token)
	if err != nil {
		return nil, err
	}
	c, err := s.company.Get(ctx)
	if err != nil {
		return nil, err
	}
	branches, err := s.openBranches(ctx, inv.BranchIDs)
	if err != nil {
		return nil, err
	}
	names := mapSlice(branches, func(b domain.Branch) string { return b.Name })
	exists, err := s.users.ExistsByEmail(ctx, inv.Email)
	if err != nil {
		return nil, err
	}
	return &iddto.InvitationInfoDTO{CompanyName: c.Name, Email: inv.Email, Role: inv.Role, Branches: names, AccountExists: exists}, nil
}

func (s *InvitationServiceImpl) Accept(ctx context.Context, token string, actor *domain.User, req iddto.AcceptInvitationRequest) (*iddto.AuthResponseDTO, error) {
	inv, err := s.open(ctx, token)
	if err != nil {
		return nil, err
	}
	var branchIDs []uint
	if inv.Role != domain.CompanyAdmin {
		branches, err := s.openBranches(ctx, inv.BranchIDs)
		if err != nil {
			return nil, err
		}
		if len(branches) == 0 {
			return nil, conflict("INVITATION_BRANCHES_CLOSED")
		}
		branchIDs = mapSlice(branches, func(b domain.Branch) uint { return b.ID })
	}

	user, err := s.users.FindByEmail(ctx, inv.Email)
	switch {
	case err == nil:
		// an existing account must prove it is the invited person by being logged in as them
		if actor == nil || actor.ID != user.ID {
			return nil, response.NewError(http.StatusUnauthorized, "LOGIN_AS_INVITED_EMAIL")
		}
		if user.RunsCompany() {
			return nil, conflict("ALREADY_IN_THE_TEAM")
		}
	case isNotFound(err):
		user, err = createUser(ctx, s.users, iddto.RegisterDTO{Name: req.Name, Email: inv.Email, Password: req.Password, Phone: req.Phone}, false)
		if err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	if inv.Role == domain.CompanyAdmin {
		if err := s.memberships.Replace(ctx, user.ID, "", nil, true); err != nil {
			return nil, err
		}
		if err := s.users.SetCompanyRole(ctx, user.ID, domain.CompanyAdmin); err != nil {
			return nil, err
		}
		user.CompanyRole = domain.CompanyAdmin
	} else if err := s.memberships.Replace(ctx, user.ID, inv.Role, branchIDs, true); err != nil {
		return nil, err
	}
	// a customer who joins the team now gets an ENO number
	if err := kit.SyncMemberNo(ctx, s.users, user); err != nil {
		return nil, err
	}
	now := s.now()
	inv.AcceptedAt = &now
	if err := s.invitations.Update(ctx, inv); err != nil {
		return nil, err
	}
	return issueTokens(user)
}

// ---------- the team's invitations (company owner and admins) ----------

func (s *InvitationServiceImpl) ListInvitations(ctx context.Context) ([]iddto.InvitationDTO, error) {
	items, err := s.invitations.ListPending(ctx, s.now())
	if err != nil {
		return nil, err
	}
	return mapSlice(items, func(inv iddomain.Invitation) iddto.InvitationDTO { return s.iv.toDTO(ctx, inv) }), nil
}

// Invite asks someone to join as an admin (owner only) or as a manager / staff member at branches
func (s *InvitationServiceImpl) Invite(ctx context.Context, actor *domain.User, req iddto.InviteRequest) (*iddto.InvitationDTO, error) {
	var ids []uint
	switch req.Role {
	case domain.CompanyAdmin:
		if !actor.IsOwner() {
			return nil, forbidden("ONLY_THE_OWNER_MANAGES_ADMINS")
		}
	case domain.MemberManager, domain.MemberStaff:
		var err error
		if ids, err = kit.OpenBranchIDs(ctx, s.branches, req.BranchIDs); err != nil {
			return nil, err
		}
	default:
		return nil, badRequest("INVALID_ROLE")
	}
	return s.iv.create(ctx, actor, req.Email, req.Role, ids)
}

func (s *InvitationServiceImpl) CancelInvitation(ctx context.Context, id uint) error {
	return s.invitations.Delete(ctx, id)
}

func (s *InvitationServiceImpl) ResendInvitation(ctx context.Context, actor *domain.User, id uint) (*iddto.InvitationDTO, error) {
	inv, err := s.invitations.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if inv.Role == domain.CompanyAdmin && !actor.IsOwner() {
		return nil, forbidden("ONLY_THE_OWNER_MANAGES_ADMINS")
	}
	return s.iv.resend(ctx, actor, inv)
}
