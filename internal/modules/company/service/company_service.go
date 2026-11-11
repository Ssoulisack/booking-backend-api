package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/modules/company/port"
	"backend/internal/platform/kit"
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
	users       output.UserRepository
	files       kit.MediaFiles
	now         func() time.Time
}

func NewCompanyService(
	company output.CompanyRepository,
	branches output.BranchRepository,
	memberships output.MembershipRepository,
	users output.UserRepository,
	storage output.FileStorage,
	media output.MediaRepository,
	mailer output.Mailer,
	frontendURL string,
) port.CompanyService {
	return &CompanyServiceImpl{
		company: company, branches: branches, memberships: memberships, users: users,
		files: kit.MediaFiles{Storage: storage, Media: media},
		now:   time.Now,
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
		LogoURL: s.files.URL(c.LogoKey), CoverURL: s.files.URL(c.CoverKey),
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
	if req.Email != "" && !kit.ValidEmail(req.Email) {
		return badRequest("INVALID_EMAIL")
	}
	if err := validSocialLinks(req.SocialLinks); err != nil {
		return err
	}
	if err := s.files.RequireOwned(ctx, 0, req.LogoKey); err != nil {
		return err
	}
	return s.files.RequireOwned(ctx, 0, req.CoverKey)
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
		ID: b.ID, Type: b.Type, Name: b.Name, Slug: b.Slug, Status: b.Status(), Address: b.Address, Province: b.Province,
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
	if req.Type == "" {
		req.Type = domain.BranchTypeSport
	}
	if !domain.ValidBranchType(req.Type) {
		return nil, badRequest("INVALID_BRANCH_TYPE")
	}
	b := domain.Branch{Name: req.Name, Slug: slug, Type: req.Type}
	if req.CloneFrom != nil {
		src, err := s.branches.GetByID(ctx, *req.CloneFrom)
		if err != nil {
			return nil, orNotFound(err, "BRANCH_NOT_FOUND")
		}
		// a draft with the same details; the pictures are not shared (each branch owns its files)
		b.Type = src.Type // the copied setup only makes sense for the same kind of place
		b.TableMode, b.HoldMinutes, b.LateMinutes = src.TableMode, src.HoldMinutes, src.LateMinutes
		b.WalkinBeforeBooking, b.MaxGuestsOnline = src.WalkinBeforeBooking, src.MaxGuestsOnline
		b.ApprovalAboveGuests, b.LastCallMinutes = src.ApprovalAboveGuests, src.LastCallMinutes
		b.BookingStyle, b.ArriveByTime, b.LateBookingMinutes = src.BookingStyle, src.ArriveByTime, src.LateBookingMinutes
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
	return kit.OpenBranchIDs(ctx, s.branches, ids)
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
	if err := s.applyRole(ctx, actor, userID, req); err != nil {
		return nil, err
	}
	return s.teamMember(ctx, userID)
}

// FindTeamCandidate looks up a registered account by phone (2055551234) or member number (CNO000042), so the owner
// or an admin can add someone who has no email to the team: they register first, then are added here.
func (s *CompanyServiceImpl) FindTeamCandidate(ctx context.Context, query string) (*dto.TeamCandidateDTO, error) {
	var user *domain.User
	var err error
	if no, ok := domain.ParseMemberNo(query); ok {
		user, err = s.users.FindByMemberNo(ctx, no)
	} else if phone, ok := domain.NormalizePhone(query); ok {
		user, err = s.users.FindByPhone(ctx, phone)
	} else {
		return nil, badRequest("ENTER_PHONE_OR_MEMBER_NO")
	}
	if err != nil {
		return nil, orNotFound(err, "ACCOUNT_NOT_FOUND")
	}
	if !user.Status {
		return nil, forbidden("ACCOUNT_DISABLED")
	}
	inTeam := user.RunsCompany()
	if !inTeam {
		if _, err := s.teamMember(ctx, user.ID); err == nil {
			inTeam = true
		}
	}
	return &dto.TeamCandidateDTO{UserID: user.ID, MemberNo: domain.MemberNoOf(user.MemberNo), Name: user.Name,
		Phone: user.Phone, Email: user.ContactEmail(), InTeam: inTeam}, nil
}

// AddTeamMember puts a registered account (a customer) on the team with a role and branches, without an
// invitation: for people who have no email. Same rules as inviting: only the owner adds admins.
func (s *CompanyServiceImpl) AddTeamMember(ctx context.Context, actor *domain.User, req dto.AddTeamMemberRequest) (*dto.TeamMemberDTO, error) {
	target, err := s.users.FindByID(ctx, req.UserID)
	if err != nil {
		return nil, orNotFound(err, "ACCOUNT_NOT_FOUND")
	}
	if !target.Status {
		return nil, forbidden("ACCOUNT_DISABLED")
	}
	if err := mayManage(actor, target); err != nil {
		return nil, err
	}
	if target.RunsCompany() {
		return nil, conflict("ALREADY_IN_THE_TEAM")
	}
	if _, err := s.teamMember(ctx, target.ID); err == nil {
		return nil, conflict("ALREADY_IN_THE_TEAM")
	}
	if err := s.applyRole(ctx, actor, target.ID, dto.TeamMemberRequest{Role: req.Role, BranchIDs: req.BranchIDs}); err != nil {
		return nil, err
	}
	return s.teamMember(ctx, target.ID)
}

// applyRole gives someone their place in the team: admin (owner only) or manager / staff at open branches; the
// ENO number follows
func (s *CompanyServiceImpl) applyRole(ctx context.Context, actor *domain.User, userID uint, req dto.TeamMemberRequest) error {
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	switch req.Role {
	case domain.CompanyAdmin:
		if !actor.IsOwner() {
			return forbidden("ONLY_THE_OWNER_MANAGES_ADMINS")
		}
		if err := s.memberships.Replace(ctx, userID, "", nil, true); err != nil {
			return err
		}
		if err := s.users.SetCompanyRole(ctx, userID, domain.CompanyAdmin); err != nil {
			return err
		}
	case domain.MemberManager, domain.MemberStaff:
		ids, err := s.openBranchIDs(ctx, req.BranchIDs)
		if err != nil {
			return err
		}
		if err := s.users.SetCompanyRole(ctx, userID, ""); err != nil {
			return err
		}
		if err := s.memberships.Replace(ctx, userID, req.Role, ids, active); err != nil {
			return err
		}
	default:
		return badRequest("INVALID_ROLE")
	}
	// a customer who joins gets an ENO number; someone already on the team keeps theirs
	if _, err := s.users.SyncMemberNo(ctx, userID); err != nil {
		return err
	}
	return nil
}

// BranchTeam lists who works at a branch: the owner and admins first, then its managers and staff. role filters
// (owner, admin, manager, staff); q finds a phone, a member number (CNO / ENO) or a name.
func (s *CompanyServiceImpl) BranchTeam(ctx context.Context, branchID uint, role, q string) ([]dto.BranchTeamMemberDTO, error) {
	out := []dto.BranchTeamMemberDTO{}
	for _, r := range []string{domain.CompanyOwner, domain.CompanyAdmin} {
		users, _, err := s.users.List(ctx, output.ListQuery{Filters: map[string]any{"company_role": r}})
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			out = append(out, dto.BranchTeamMemberDTO{UserID: u.ID, MemberNo: domain.MemberNoOf(u.MemberNo), Name: u.Name,
				Phone: u.Phone, Email: u.ContactEmail(), Role: r, Active: u.Status})
		}
	}
	members, err := s.memberships.ListByBranch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if m.User.RunsCompany() {
			continue // listed above
		}
		out = append(out, dto.BranchTeamMemberDTO{UserID: m.UserID, MemberNo: domain.MemberNoOf(m.User.MemberNo),
			Name: m.User.Name, Phone: m.User.Phone, Email: m.User.ContactEmail(), Role: m.Role, Active: m.Active && m.User.Status})
	}
	q = strings.TrimSpace(q)
	phone, isPhone := domain.NormalizePhone(q)
	no, isNo := domain.ParseMemberNo(q)
	keep := out[:0]
	for _, p := range out {
		if role != "" && p.Role != role {
			continue
		}
		if q != "" {
			hit := strings.Contains(strings.ToLower(p.Name), strings.ToLower(q)) ||
				strings.Contains(p.Phone, q) || strings.Contains(p.MemberNo, strings.ToUpper(q)) ||
				(isPhone && p.Phone == phone) || (isNo && p.MemberNo == no)
			if !hit {
				continue
			}
		}
		keep = append(keep, p)
	}
	return keep, nil
}

// branchMember is someone's place at one branch, checked before a change there: only managers and staff are changed
// per branch (the owner and admins on the company Team page)
func (s *CompanyServiceImpl) branchMember(ctx context.Context, actor *domain.User, branchID, userID uint) error {
	target, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return orNotFound(err, "TEAM_MEMBER_NOT_FOUND")
	}
	if err := mayManage(actor, target); err != nil {
		return err
	}
	if target.RunsCompany() {
		return badRequest("MANAGE_ADMINS_ON_TEAM_PAGE")
	}
	m, err := s.memberships.Get(ctx, branchID, userID)
	if err != nil {
		return err
	}
	if m == nil {
		return notFound("TEAM_MEMBER_NOT_FOUND")
	}
	return nil
}

// SetBranchAccess switches someone's access to one branch off (or back on): off, the branch refuses them at once
// and they see the site as a customer there
func (s *CompanyServiceImpl) SetBranchAccess(ctx context.Context, actor *domain.User, branchID, userID uint, active bool) error {
	if err := s.branchMember(ctx, actor, branchID, userID); err != nil {
		return err
	}
	return s.memberships.SetActive(ctx, branchID, userID, active)
}

// RemoveFromBranch takes someone off one branch; with no branch left they are a customer again (a CNO number)
func (s *CompanyServiceImpl) RemoveFromBranch(ctx context.Context, actor *domain.User, branchID, userID uint) error {
	if err := s.branchMember(ctx, actor, branchID, userID); err != nil {
		return err
	}
	if err := s.memberships.Delete(ctx, branchID, userID); err != nil {
		return err
	}
	_, err := s.users.SyncMemberNo(ctx, userID)
	return err
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

// ---------- users ----------

func (s *CompanyServiceImpl) ListUsers(ctx context.Context, q dto.ListQuery) (*dto.PagedResult[dto.UserDTO], error) {
	lq, page, limit := newListQuery(q)
	users, total, err := s.users.List(ctx, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(users, total, page, limit, func(u domain.User) dto.UserDTO { return kit.UserDTO(&u) }), nil
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
	if err := kit.ValidPassword(password); err != nil {
		return err
	}
	if err := s.mayAct(ctx, actor, targetID); err != nil {
		return err
	}
	hash, err := utils.Encrypt(password)
	if err != nil {
		return err
	}
	// the person did not choose it: they are asked to set their own after logging in
	return s.users.SetPasswordByTeam(ctx, targetID, hash, s.now())
}
