package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	iddto "backend/internal/modules/identity/dto"
	"backend/internal/modules/identity/port"
	"backend/internal/platform/kit"
	"backend/internal/platform/redis"
	"backend/pkg/jwt"
	"backend/pkg/response"
	"backend/pkg/utils"
	"context"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const (
	resetTokenTTL = time.Hour
)

type AuthServiceImpl struct {
	userRepo    output.UserRepository
	cacheRepo   redis.CacheRepository
	memberships output.MembershipRepository
	branches    output.BranchRepository
	resets      output.PasswordResetRepository
	social      output.SocialVerifier
	socialIDs   port.SocialIdentityRepository
	mailer      output.Mailer
	frontendURL string
	now         func() time.Time
}

func NewAuthService(
	userRepo output.UserRepository,
	cacheRepo redis.CacheRepository,
	memberships output.MembershipRepository,
	branches output.BranchRepository,
	resets output.PasswordResetRepository,
	social output.SocialVerifier,
	socialIDs port.SocialIdentityRepository,
	mailer output.Mailer,
	frontendURL string,
) port.AuthService {
	return &AuthServiceImpl{
		userRepo:    userRepo,
		cacheRepo:   cacheRepo,
		memberships: memberships,
		branches:    branches,
		resets:      resets,
		social:      social,
		socialIDs:   socialIDs,
		mailer:      mailer,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		now:         time.Now,
	}
}

// Register creates a customer account. Branch members join through invitations.
func (s *AuthServiceImpl) Register(ctx context.Context, req iddto.RegisterDTO) (*iddto.AuthResponseDTO, error) {
	user, err := createUser(ctx, s.userRepo, req, true)
	if err != nil {
		return nil, err
	}
	if err := kit.SyncMemberNo(ctx, s.userRepo, user); err != nil {
		return nil, err
	}
	return issueTokens(user)
}

// identity checks the contact details of an account (new, or exceptID when changing one) and returns them
// normalized. An account phone is always a Lao mobile number, stored as 20xxxxxxxx. With requirePhone (a new
// registration) a phone is required; without a phone the account needs a real email. Without a real email an
// account gets a stand-in one made from its phone (it can log in by phone, but never reset by email).
func identity(ctx context.Context, repo output.UserRepository, exceptID uint, requirePhone bool,
	phoneIn, emailIn string) (email, phone string, generated bool, err error) {
	if strings.TrimSpace(phoneIn) != "" {
		var ok bool
		if phone, ok = domain.NormalizePhone(phoneIn); !ok || !domain.IsLaoPhone(phone) {
			return "", "", false, badRequest("INVALID_PHONE")
		}
		taken, err := repo.PhoneTaken(ctx, phone, exceptID)
		if err != nil {
			return "", "", false, err
		}
		if taken {
			return "", "", false, conflict("PHONE_ALREADY_REGISTERED")
		}
	} else if requirePhone {
		return "", "", false, badRequest("PHONE_REQUIRED") // the team reaches a customer by phone
	}

	email = kit.NormalizeEmail(emailIn)
	if email == "" || domain.IsGeneratedEmail(email) {
		if phone == "" {
			return "", "", false, badRequest("EMAIL_REQUIRED") // neither a phone nor an email: no way to reach or find them
		}
		return domain.GeneratedEmail(phone), phone, true, nil
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return "", "", false, badRequest("INVALID_EMAIL")
	}
	taken, err := repo.EmailTaken(ctx, email, exceptID)
	if err != nil {
		return "", "", false, err
	}
	if taken {
		return "", "", false, conflict("EMAIL_ALREADY_REGISTERED")
	}
	return email, phone, false, nil
}

// createUser validates a registration and stores a plain account (no company role).
// A public registration (requirePhone) must give a phone; an account made from an invitation may add it later.
func createUser(ctx context.Context, repo output.UserRepository, req iddto.RegisterDTO, requirePhone bool) (*domain.User, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, badRequest("NAME_IS_REQUIRED")
	}
	if err := kit.ValidPassword(req.Password); err != nil {
		return nil, err
	}
	email, phone, generated, err := identity(ctx, repo, 0, requirePhone, req.Phone, req.Email)
	if err != nil {
		return nil, err
	}

	hash, err := utils.Encrypt(req.Password)
	if err != nil {
		return nil, err
	}
	user := &domain.User{
		Email:          email,
		Username:       email, // username is unique but accounts are identified by email or phone
		Name:           strings.TrimSpace(req.Name),
		Phone:          phone,
		EmailGenerated: generated,
		PasswordHash:   hash,
		Status:         true,
		LastActive:     time.Now(),
	}
	if err := repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateProfile changes the logged-in person's name and contact details (same rules as registering)
func (s *AuthServiceImpl) UpdateProfile(ctx context.Context, userID uint, req iddto.ProfileRequest) (*dto.UserDTO, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, badRequest("NAME_IS_REQUIRED")
	}
	email, phone, generated, err := identity(ctx, s.userRepo, user.ID, false, req.Phone, req.Email)
	if err != nil {
		return nil, err
	}
	user.Name, user.Email, user.Username, user.Phone = strings.TrimSpace(req.Name), email, email, phone
	user.EmailGenerated = generated
	if err := s.userRepo.UpdateProfile(ctx, user); err != nil {
		return nil, err
	}
	d := kit.UserDTO(user)
	return &d, nil
}

func (s *AuthServiceImpl) Login(ctx context.Context, req iddto.LoginDTO) (*iddto.AuthResponseDTO, error) {
	invalid := response.NewError(http.StatusUnauthorized, "INVALID_CREDENTIALS")

	login := strings.TrimSpace(req.Login)
	if login == "" {
		login = req.Email
	}
	var user *domain.User
	var err error
	if strings.Contains(login, "@") {
		user, err = s.userRepo.FindByEmail(ctx, kit.NormalizeEmail(login))
	} else if phone, ok := domain.NormalizePhone(login); ok {
		user, err = s.userRepo.FindByPhone(ctx, phone)
	} else {
		return nil, invalid
	}
	if err != nil {
		if isNotFound(err) {
			return nil, invalid
		}
		return nil, err
	}
	if utils.VerifyPassword(string(user.PasswordHash), req.Password) != nil {
		return nil, invalid
	}
	if !user.Status {
		return nil, forbidden("ACCOUNT_DISABLED")
	}
	return issueTokens(user)
}

// checkUsable refuses disabled accounts and tokens issued before the last password reset
func checkUsable(u *domain.User, issuedAt int64) error {
	if !u.Status {
		return forbidden("ACCOUNT_DISABLED")
	}
	if u.PasswordChangedAt != nil && issuedAt < u.PasswordChangedAt.Unix() {
		return response.NewError(http.StatusUnauthorized, "TOKEN_REVOKED")
	}
	return nil
}

// Refresh re-reads the user so a disabled account or a password reset takes effect
func (s *AuthServiceImpl) Refresh(ctx context.Context, userID uint, issuedAt int64) (*iddto.AuthResponseDTO, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := checkUsable(user, issuedAt); err != nil {
		return nil, err
	}
	return issueTokens(user)
}

func (s *AuthServiceImpl) Me(ctx context.Context, userID uint) (*dto.UserDTO, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	d := kit.UserDTO(user)
	return &d, nil
}

// MyBranches is the branch switcher: every branch for the owner and admins, else where they work
func (s *AuthServiceImpl) MyBranches(ctx context.Context, userID uint) ([]dto.MyBranchDTO, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.RunsCompany() {
		branches, _, err := s.branches.List(ctx, output.ListQuery{Filters: map[string]any{}})
		if err != nil {
			return nil, err
		}
		return mapSlice(branches, func(b domain.Branch) dto.MyBranchDTO {
			return dto.MyBranchDTO{
				BranchID: b.ID, Type: b.Type, Name: b.Name, Slug: b.Slug, Status: b.Status(), Role: user.CompanyRole, Active: true,
			}
		}), nil
	}
	memberships, err := s.memberships.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapSlice(memberships, func(m domain.Membership) dto.MyBranchDTO {
		return dto.MyBranchDTO{
			BranchID: m.BranchID, Type: m.Branch.Type, Name: m.Branch.Name, Slug: m.Branch.Slug,
			Status: m.Branch.Status(), Role: m.Role, Active: m.Active,
		}
	}), nil
}

// ForgotPassword emails a one-hour reset link. Unknown or disabled accounts get no email,
// but the caller sees the same answer either way.
func (s *AuthServiceImpl) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.userRepo.FindByEmail(ctx, kit.NormalizeEmail(email))
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	if !user.Status || user.EmailGenerated { // no real address to send to: the page explains what to do
		return nil
	}
	raw, hash, err := newToken()
	if err != nil {
		return err
	}
	reset := domain.PasswordReset{UserID: user.ID, TokenHash: hash, ExpiresAt: s.now().Add(resetTokenTTL)}
	if err := s.resets.Create(ctx, &reset); err != nil {
		return err
	}
	if err := s.mailer.SendPasswordReset(user.Email, user.Name, s.frontendURL+"/reset-password/"+raw); err != nil {
		log.Printf("❌ password reset email to user %d failed: %v", user.ID, err)
	}
	return nil
}

// ResetPassword sets a new password from a valid link; every older token stops working
func (s *AuthServiceImpl) ResetPassword(ctx context.Context, req iddto.ResetPasswordRequest) error {
	if err := kit.ValidPassword(req.Password); err != nil {
		return err
	}
	reset, err := s.resets.GetByTokenHash(ctx, hashToken(req.Token))
	if err != nil {
		return err
	}
	now := s.now()
	if reset == nil || reset.UsedAt != nil || now.After(reset.ExpiresAt) {
		return badRequest("INVALID_OR_EXPIRED_TOKEN")
	}
	hash, err := utils.Encrypt(req.Password)
	if err != nil {
		return err
	}
	if err := s.userRepo.SetPassword(ctx, reset.UserID, hash, now); err != nil {
		return err
	}
	return s.resets.UseAll(ctx, reset.UserID, now)
}

func issueTokens(user *domain.User) (*iddto.AuthResponseDTO, error) {
	// role claim unused: roles are read from the database on every request
	pair, err := jwt.GenerateJWTToken(int(user.ID), 0)
	if err != nil {
		return nil, err
	}
	return &iddto.AuthResponseDTO{
		// TokenPair holds the tokens as quoted JSON strings
		AccessToken:  strings.Trim(string(pair.AccessToken), `"`),
		RefreshToken: strings.Trim(string(pair.RefreshToken), `"`),
		User:         kit.UserDTO(user),
	}, nil
}
