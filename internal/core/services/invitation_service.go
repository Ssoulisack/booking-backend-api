package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
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
	invitations output.InvitationRepository
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

func (iv invites) toDTO(ctx context.Context, inv domain.Invitation) dto.InvitationDTO {
	return dto.InvitationDTO{
		ID: inv.ID, Email: inv.Email, Role: inv.Role, Branches: iv.branchRefs(ctx, inv.BranchIDs), ExpiresAt: inv.ExpiresAt,
	}
}

// where says in the email where the person will work
func (iv invites) where(ctx context.Context, inv domain.Invitation) string {
	if inv.Role == domain.CompanyAdmin {
		return "every branch"
	}
	names := []string{}
	for _, r := range iv.branchRefs(ctx, inv.BranchIDs) {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}

func (iv invites) send(ctx context.Context, inv domain.Invitation, actor *domain.User, raw string) error {
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
func (iv invites) create(ctx context.Context, actor *domain.User, email, role string, branchIDs []uint) (*dto.InvitationDTO, error) {
	email = normalizeEmail(email)
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
	inv := domain.Invitation{
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
func (iv invites) resend(ctx context.Context, actor *domain.User, inv *domain.Invitation) (*dto.InvitationDTO, error) {
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
	invitations output.InvitationRepository
	memberships output.MembershipRepository
	branches    output.BranchRepository
	company     output.CompanyRepository
	users       output.UserRepository
	now         func() time.Time
}

func NewInvitationService(
	invitations output.InvitationRepository,
	memberships output.MembershipRepository,
	branches output.BranchRepository,
	company output.CompanyRepository,
	users output.UserRepository,
) input.InvitationService {
	return &InvitationServiceImpl{
		invitations: invitations, memberships: memberships, branches: branches, company: company, users: users, now: time.Now,
	}
}

// open finds a usable invitation; used, expired and unknown links look the same
func (s *InvitationServiceImpl) open(ctx context.Context, token string) (*domain.Invitation, error) {
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

func (s *InvitationServiceImpl) GetInvitation(ctx context.Context, token string) (*dto.InvitationInfoDTO, error) {
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
	return &dto.InvitationInfoDTO{CompanyName: c.Name, Email: inv.Email, Role: inv.Role, Branches: names, AccountExists: exists}, nil
}

func (s *InvitationServiceImpl) Accept(ctx context.Context, token string, actor *domain.User, req dto.AcceptInvitationRequest) (*dto.AuthResponseDTO, error) {
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
		user, err = createUser(ctx, s.users, dto.RegisterDTO{Name: req.Name, Email: inv.Email, Password: req.Password, Phone: req.Phone}, false)
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
	now := s.now()
	inv.AcceptedAt = &now
	if err := s.invitations.Update(ctx, inv); err != nil {
		return nil, err
	}
	return issueTokens(user)
}
