package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"backend/pkg/utils"
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// CompanyServiceImpl is what the owner and admins run. The owner alone manages admins and
// ownership; admins manage branches, managers and staff.
type CompanyServiceImpl struct {
	company     output.CompanyRepository
	branches    output.BranchRepository
	memberships output.MembershipRepository
	invitations output.InvitationRepository
	users       output.UserRepository
	files       mediaFiles
	invites     invites
	now         func() time.Time
}

func NewCompanyService(
	company output.CompanyRepository,
	branches output.BranchRepository,
	memberships output.MembershipRepository,
	invitations output.InvitationRepository,
	users output.UserRepository,
	storage output.FileStorage,
	media output.MediaRepository,
	mailer output.Mailer,
	frontendURL string,
) input.CompanyService {
	return &CompanyServiceImpl{
		company: company, branches: branches, memberships: memberships, invitations: invitations, users: users,
		files: mediaFiles{storage: storage, media: media},
		invites: invites{
			invitations: invitations, memberships: memberships, branches: branches, company: company, users: users,
			mailer: mailer, frontendURL: strings.TrimRight(frontendURL, "/"), now: time.Now,
		},
		now: time.Now,
	}
}

// ---------- company settings ----------

func (s *CompanyServiceImpl) toCompanyDTO(c *domain.Company) *dto.CompanyDTO {
	return &dto.CompanyDTO{
		CompanyRequest: dto.CompanyRequest{
			Name: c.Name, LogoKey: c.LogoKey, CoverKey: c.CoverKey, BrandColor: c.BrandColor,
			RingColors: ringColorsOrEmpty(c.RingColors), HeadingFont: c.HeadingFont, BodyFont: c.BodyFont,
			Description: c.Description, Phone: c.Phone, Email: c.Email, SocialLinks: c.SocialLinks,
			Language: c.Language, Currency: c.Currency, Timezone: c.Timezone,
			BookingMode: c.BookingMode, BookingPolicy: c.BookingPolicy, RequireApproval: c.RequireApproval,
		},
		LogoURL: s.files.url(c.LogoKey), CoverURL: s.files.url(c.CoverKey),
	}
}

func (s *CompanyServiceImpl) GetCompany(ctx context.Context) (*dto.CompanyDTO, error) {
	c, err := s.company.Get(ctx)
	if err != nil {
		return nil, err
	}
	return s.toCompanyDTO(c), nil
}

func (s *CompanyServiceImpl) validateCompany(ctx context.Context, req *dto.CompanyRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	req.Email = strings.TrimSpace(req.Email)
	switch {
	case req.Name == "":
		return badRequest("NAME_IS_REQUIRED")
	case !validCurrency(req.Currency):
		return badRequest("INVALID_CURRENCY")
	case !languages[req.Language]:
		return badRequest("INVALID_LANGUAGE")
	case !validTimezone(req.Timezone):
		return badRequest("INVALID_TIMEZONE")
	case req.BrandColor != "" && !brandColorPattern.MatchString(req.BrandColor):
		return badRequest("INVALID_BRAND_COLOR")
	case !bookingModes[req.BookingMode]:
		return badRequest("INVALID_BOOKING_MODE")
	case !validLook(req.RingColors, req.HeadingFont, req.BodyFont):
		return badRequest("INVALID_LOOK")
	case utf8.RuneCountInString(req.Description) > maxDescriptionChars:
		return badRequest("DESCRIPTION_TOO_LONG")
	case utf8.RuneCountInString(req.BookingPolicy) > maxBookingPolicyChar:
		return badRequest("BOOKING_POLICY_TOO_LONG")
	}
	if req.Email != "" && !validEmail(req.Email) {
		return badRequest("INVALID_EMAIL")
	}
	if err := validSocialLinks(req.SocialLinks); err != nil {
		return err
	}
	if err := s.files.requireOwned(ctx, 0, req.LogoKey); err != nil {
		return err
	}
	return s.files.requireOwned(ctx, 0, req.CoverKey)
}

// UpdateCompany replaces every company setting at once; branches follow unless they override
func (s *CompanyServiceImpl) UpdateCompany(ctx context.Context, req dto.CompanyRequest) (*dto.CompanyDTO, error) {
	c, err := s.company.Get(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.validateCompany(ctx, &req); err != nil {
		return nil, err
	}
	c.Name, c.LogoKey, c.CoverKey, c.BrandColor = req.Name, req.LogoKey, req.CoverKey, req.BrandColor
	c.RingColors, c.HeadingFont, c.BodyFont = req.RingColors, req.HeadingFont, req.BodyFont
	c.Description, c.Phone, c.Email, c.SocialLinks = req.Description, strings.TrimSpace(req.Phone), req.Email, req.SocialLinks
	c.Language, c.Currency, c.Timezone = req.Language, req.Currency, req.Timezone
	c.BookingMode, c.BookingPolicy, c.RequireApproval = req.BookingMode, req.BookingPolicy, req.RequireApproval
	if err := s.company.Update(ctx, c); err != nil {
		return nil, err
	}
	return s.toCompanyDTO(c), nil
}

// ---------- branches ----------

func toBranchDTO(b domain.Branch) dto.BranchDTO {
	return dto.BranchDTO{
		ID: b.ID, Name: b.Name, Slug: b.Slug, Status: b.Status(), Address: b.Address, Province: b.Province,
		PublicPath: "/b/" + b.Slug, PublishedAt: b.PublishedAt, CreatedAt: b.CreatedAt,
	}
}

func (s *CompanyServiceImpl) ListBranches(ctx context.Context) ([]dto.BranchDTO, error) {
	items, _, err := s.branches.List(ctx, output.ListQuery{Filters: map[string]any{}})
	if err != nil {
		return nil, err
	}
	return mapSlice(items, toBranchDTO), nil
}

func (s *CompanyServiceImpl) CreateBranch(ctx context.Context, req dto.CreateBranchRequest) (*dto.BranchDTO, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, badRequest("NAME_IS_REQUIRED")
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = slugify(req.Name, "branch")
	}
	if !validSlug(slug) {
		return nil, badRequest("INVALID_SLUG")
	}
	taken, err := s.branches.SlugTaken(ctx, slug, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, conflict("SLUG_TAKEN")
	}
	b := domain.Branch{Name: req.Name, Slug: slug}
	if req.CloneFrom != nil {
		src, err := s.branches.GetByID(ctx, *req.CloneFrom)
		if err != nil {
			return nil, orNotFound(err, "BRANCH_NOT_FOUND")
		}
		// a draft with the same details; the pictures are not shared (each branch owns its files)
		b.Description, b.Address, b.Province, b.Phone, b.MapURL = src.Description, src.Address, src.Province, src.Phone, src.MapURL
		b.SocialLinks = src.SocialLinks
		b.OwnTimezone, b.OwnBookingMode, b.OwnBookingPolicy, b.OwnRequireApproval =
			src.OwnTimezone, src.OwnBookingMode, src.OwnBookingPolicy, src.OwnRequireApproval
		b.OwnRingColors, b.OwnHeadingFont, b.OwnBodyFont = src.OwnRingColors, src.OwnHeadingFont, src.OwnBodyFont
		if err := s.branches.Clone(ctx, src.ID, &b); err != nil {
			return nil, err
		}
	} else if err := s.branches.Create(ctx, &b); err != nil {
		return nil, err
	}
	d := toBranchDTO(b)
	return &d, nil
}

// SetClosed closes a branch (its page goes offline, its team loses access) or reopens it as a draft
func (s *CompanyServiceImpl) SetClosed(ctx context.Context, branchID uint, closed bool) (*dto.BranchDTO, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, orNotFound(err, "BRANCH_NOT_FOUND")
	}
	b.Closed = closed
	if closed {
		b.Published = false
	}
	if err := s.branches.Update(ctx, b); err != nil {
		return nil, err
	}
	d := toBranchDTO(*b)
	return &d, nil
}

// ---------- team ----------

func teamRank(role string) int {
	switch role {
	case domain.CompanyOwner:
		return 0
	case domain.CompanyAdmin:
		return 1
	case domain.MemberManager:
		return 2
	}
	return 3
}

func (s *CompanyServiceImpl) ListTeam(ctx context.Context) ([]dto.TeamMemberDTO, error) {
	team := []dto.TeamMemberDTO{}
	for _, role := range []string{domain.CompanyOwner, domain.CompanyAdmin} {
		users, _, err := s.users.List(ctx, output.ListQuery{Filters: map[string]any{"company_role": role}})
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			team = append(team, dto.TeamMemberDTO{
				UserID: u.ID, MemberNo: domain.MemberNoOf(u.MemberNo), Name: u.Name, Email: u.ContactEmail(), Phone: u.Phone, Role: role,
				Branches: []dto.BranchRef{}, Active: true,
			})
		}
	}
	all, err := s.memberships.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	byUser := map[uint]int{}
	for _, m := range all {
		i, seen := byUser[m.UserID]
		if !seen {
			team = append(team, dto.TeamMemberDTO{
				UserID: m.UserID, MemberNo: domain.MemberNoOf(m.User.MemberNo), Name: m.User.Name, Email: m.User.ContactEmail(), Phone: m.User.Phone,
				Role: m.Role, Branches: []dto.BranchRef{}, Active: m.Active,
			})
			i = len(team) - 1
			byUser[m.UserID] = i
		}
		team[i].Branches = append(team[i].Branches, dto.BranchRef{ID: m.BranchID, Name: m.Branch.Name})
	}
	sort.SliceStable(team, func(a, b int) bool { return teamRank(team[a].Role) < teamRank(team[b].Role) })
	return team, nil
}

func (s *CompanyServiceImpl) teamMember(ctx context.Context, userID uint) (*dto.TeamMemberDTO, error) {
	team, err := s.ListTeam(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range team {
		if m.UserID == userID {
			return &m, nil
		}
	}
	return nil, notFound("TEAM_MEMBER_NOT_FOUND")
}

// openBranchIDs checks that every id is an existing, open branch (at least one, no repeats)
func (s *CompanyServiceImpl) openBranchIDs(ctx context.Context, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, badRequest("CHOOSE_AT_LEAST_ONE_BRANCH")
	}
	seen := map[uint]bool{}
	out := []uint{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		b, err := s.branches.GetByID(ctx, id)
		if err != nil {
			return nil, orBadRequest(err, "BRANCH_NOT_FOUND")
		}
		if b.Closed {
			return nil, badRequest("BRANCH_CLOSED")
		}
		out = append(out, id)
	}
	return out, nil
}

// mayManage says whether actor may change target's place in the team: nobody changes the
// owner or themselves this way, and only the owner manages admins
func mayManage(actor, target *domain.User) error {
	switch {
	case actor.ID == target.ID:
		return badRequest("CANNOT_CHANGE_YOURSELF")
	case target.IsOwner():
		return badRequest("USE_TRANSFER_OWNERSHIP")
	case target.CompanyRole == domain.CompanyAdmin && !actor.IsOwner():
		return forbidden("ONLY_THE_OWNER_MANAGES_ADMINS")
	}
	return nil
}

func (s *CompanyServiceImpl) UpdateTeamMember(ctx context.Context, actor *domain.User, userID uint, req dto.TeamMemberRequest) (*dto.TeamMemberDTO, error) {
	target, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, orNotFound(err, "TEAM_MEMBER_NOT_FOUND")
	}
	if err := mayManage(actor, target); err != nil {
		return nil, err
	}
	if _, err := s.teamMember(ctx, userID); err != nil {
		return nil, err
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	switch req.Role {
	case domain.CompanyAdmin:
		if !actor.IsOwner() {
			return nil, forbidden("ONLY_THE_OWNER_MANAGES_ADMINS")
		}
		if err := s.memberships.Replace(ctx, userID, "", nil, true); err != nil {
			return nil, err
		}
		if err := s.users.SetCompanyRole(ctx, userID, domain.CompanyAdmin); err != nil {
			return nil, err
		}
	case domain.MemberManager, domain.MemberStaff:
		ids, err := s.openBranchIDs(ctx, req.BranchIDs)
		if err != nil {
			return nil, err
		}
		if err := s.users.SetCompanyRole(ctx, userID, ""); err != nil {
			return nil, err
		}
		if err := s.memberships.Replace(ctx, userID, req.Role, ids, active); err != nil {
			return nil, err
		}
	default:
		return nil, badRequest("INVALID_ROLE")
	}
	// still on the team (admin <-> manager / staff): the ENO number stays
	if _, err := s.users.SyncMemberNo(ctx, userID); err != nil {
		return nil, err
	}
	return s.teamMember(ctx, userID)
}

func (s *CompanyServiceImpl) RemoveTeamMember(ctx context.Context, actor *domain.User, userID uint) error {
	target, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return orNotFound(err, "TEAM_MEMBER_NOT_FOUND")
	}
	if err := mayManage(actor, target); err != nil {
		return err
	}
	if _, err := s.teamMember(ctx, userID); err != nil {
		return err
	}
	if err := s.users.SetCompanyRole(ctx, userID, ""); err != nil {
		return err
	}
	if err := s.memberships.Replace(ctx, userID, "", nil, true); err != nil {
		return err
	}
	// back to being a customer: the next CNO number
	_, err = s.users.SyncMemberNo(ctx, userID)
	return err
}

func (s *CompanyServiceImpl) TransferOwnership(ctx context.Context, actor *domain.User, toUserID uint) error {
	if !actor.IsOwner() {
		return forbidden("ONLY_THE_OWNER")
	}
	if actor.ID == toUserID {
		return badRequest("ALREADY_THE_OWNER")
	}
	target, err := s.users.FindByID(ctx, toUserID)
	if err != nil {
		return orNotFound(err, "TEAM_MEMBER_NOT_FOUND")
	}
	if target.CompanyRole != domain.CompanyAdmin {
		return badRequest("NEW_OWNER_MUST_BE_AN_ADMIN")
	}
	return s.users.TransferOwnership(ctx, actor.ID, toUserID)
}

// ---------- invitations ----------

func (s *CompanyServiceImpl) ListInvitations(ctx context.Context) ([]dto.InvitationDTO, error) {
	items, err := s.invitations.ListPending(ctx, s.now())
	if err != nil {
		return nil, err
	}
	return mapSlice(items, func(inv domain.Invitation) dto.InvitationDTO { return s.invites.toDTO(ctx, inv) }), nil
}

// Invite asks someone to join as an admin (owner only) or as a manager / staff member at branches
func (s *CompanyServiceImpl) Invite(ctx context.Context, actor *domain.User, req dto.InviteRequest) (*dto.InvitationDTO, error) {
	var ids []uint
	switch req.Role {
	case domain.CompanyAdmin:
		if !actor.IsOwner() {
			return nil, forbidden("ONLY_THE_OWNER_MANAGES_ADMINS")
		}
	case domain.MemberManager, domain.MemberStaff:
		var err error
		if ids, err = s.openBranchIDs(ctx, req.BranchIDs); err != nil {
			return nil, err
		}
	default:
		return nil, badRequest("INVALID_ROLE")
	}
	return s.invites.create(ctx, actor, req.Email, req.Role, ids)
}

func (s *CompanyServiceImpl) CancelInvitation(ctx context.Context, id uint) error {
	return s.invitations.Delete(ctx, id)
}

func (s *CompanyServiceImpl) ResendInvitation(ctx context.Context, actor *domain.User, id uint) (*dto.InvitationDTO, error) {
	inv, err := s.invitations.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if inv.Role == domain.CompanyAdmin && !actor.IsOwner() {
		return nil, forbidden("ONLY_THE_OWNER_MANAGES_ADMINS")
	}
	return s.invites.resend(ctx, actor, inv)
}

// ---------- users ----------

func (s *CompanyServiceImpl) ListUsers(ctx context.Context, q dto.ListQuery) (*dto.PagedResult[dto.UserDTO], error) {
	lq, page, limit := newListQuery(q)
	users, total, err := s.users.List(ctx, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(users, total, page, limit, func(u domain.User) dto.UserDTO { return toUserDTO(&u) }), nil
}

// mayAct: nobody acts on their own account or on the owner; only the owner on an admin
func (s *CompanyServiceImpl) mayAct(ctx context.Context, actor *domain.User, targetID uint) error {
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	switch {
	case actor.ID == target.ID:
		return badRequest("CANNOT_CHANGE_YOURSELF")
	case target.IsOwner():
		return forbidden("NOT_ALLOWED_FOR_THE_OWNER")
	case target.CompanyRole == domain.CompanyAdmin && !actor.IsOwner():
		return forbidden("ONLY_THE_OWNER_MANAGES_ADMINS")
	}
	return nil
}

// SetUserStatus disables or enables an account; it takes effect on the next request
func (s *CompanyServiceImpl) SetUserStatus(ctx context.Context, actor *domain.User, targetID uint, status bool) error {
	if err := s.mayAct(ctx, actor, targetID); err != nil {
		return err
	}
	return s.users.SetStatus(ctx, targetID, status)
}

func (s *CompanyServiceImpl) SetUserPassword(ctx context.Context, actor *domain.User, targetID uint, password string) error {
	if err := validPassword(password); err != nil {
		return err
	}
	if err := s.mayAct(ctx, actor, targetID); err != nil {
		return err
	}
	hash, err := utils.Encrypt(password)
	if err != nil {
		return err
	}
	return s.users.SetPassword(ctx, targetID, hash, s.now())
}
