package services

import (
	"backend/internal/adapters/secondary/redis"
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"backend/pkg/jwt"
	"backend/pkg/response"
	"backend/pkg/utils"
	"context"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const (
	minPasswordLength = 8
	resetTokenTTL     = time.Hour
)

type AuthServiceImpl struct {
	userRepo    output.UserRepository
	cacheRepo   redis.CacheRepository
	memberships output.MembershipRepository
	branches    output.BranchRepository
	resets      output.PasswordResetRepository
	social      output.SocialVerifier
	socialIDs   output.SocialIdentityRepository
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
	socialIDs output.SocialIdentityRepository,
	mailer output.Mailer,
	frontendURL string,
) input.AuthService {
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
func (s *AuthServiceImpl) Register(ctx context.Context, req dto.RegisterDTO) (*dto.AuthResponseDTO, error) {
	user, err := createUser(ctx, s.userRepo, req, true)
	if err != nil {
		return nil, err
	}
	if err := syncMemberNo(ctx, s.userRepo, user); err != nil {
		return nil, err
	}
	return issueTokens(user)
}

// syncMemberNo gives the account the member number for its role (CNO customer, ENO team) and
// keeps it on u for the response. Call it once an account is created or its role has changed.
func syncMemberNo(ctx context.Context, users output.UserRepository, u *domain.User) error {
	no, err := users.SyncMemberNo(ctx, u.ID)
	if err != nil {
		return err
	}
	u.MemberNo = &no
	return nil
}

func validPassword(password string) error {
	if len(password) < minPasswordLength {
		return badRequest("PASSWORD_TOO_SHORT")
	}
	return nil
}

// identity checks the contact details of an account (new, or exceptID when changing one) and
// returns them normalized. With needNationality the nationality must be chosen; "LA" needs a phone
// and gets a stand-in email when none is given, any other nationality needs a real email.
// An account phone is always a Lao mobile number, stored as 20xxxxxxxx, whatever the nationality:
// foreigners add one if they have a Lao number, otherwise they leave it empty.
func identity(ctx context.Context, repo output.UserRepository, exceptID uint, needNationality bool,
	nationality, phoneIn, emailIn string) (email, phone string, generated bool, err error) {
	nationality = strings.ToUpper(strings.TrimSpace(nationality))
	switch {
	case nationality == "" && needNationality:
		return "", "", false, badRequest("NATIONALITY_REQUIRED")
	case nationality != "" && !domain.ValidNationality(nationality):
		return "", "", false, badRequest("INVALID_NATIONALITY")
	}
	lao := nationality == domain.NationalityLaos

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
	} else if lao {
		return "", "", false, badRequest("PHONE_REQUIRED")
	}

	email = normalizeEmail(emailIn)
	if email == "" || domain.IsGeneratedEmail(email) {
		if !lao {
			return "", "", false, badRequest("EMAIL_REQUIRED")
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
// Accounts made from an invitation (needNationality false) choose their nationality on first login.
func createUser(ctx context.Context, repo output.UserRepository, req dto.RegisterDTO, needNationality bool) (*domain.User, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, badRequest("NAME_IS_REQUIRED")
	}
	if err := validPassword(req.Password); err != nil {
		return nil, err
	}
	email, phone, generated, err := identity(ctx, repo, 0, needNationality, req.Nationality, req.Phone, req.Email)
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
		Nationality:    strings.ToUpper(strings.TrimSpace(req.Nationality)),
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
func (s *AuthServiceImpl) UpdateProfile(ctx context.Context, userID uint, req dto.ProfileRequest) (*dto.UserDTO, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, badRequest("NAME_IS_REQUIRED")
	}
	email, phone, generated, err := identity(ctx, s.userRepo, user.ID, true, req.Nationality, req.Phone, req.Email)
	if err != nil {
		return nil, err
	}
	user.Name, user.Email, user.Username, user.Phone = strings.TrimSpace(req.Name), email, email, phone
	user.Nationality, user.EmailGenerated = strings.ToUpper(strings.TrimSpace(req.Nationality)), generated
	if err := s.userRepo.UpdateProfile(ctx, user); err != nil {
		return nil, err
	}
	d := toUserDTO(user)
	return &d, nil
}

func (s *AuthServiceImpl) Login(ctx context.Context, req dto.LoginDTO) (*dto.AuthResponseDTO, error) {
	invalid := response.NewError(http.StatusUnauthorized, "INVALID_CREDENTIALS")

	login := strings.TrimSpace(req.Login)
	if login == "" {
		login = req.Email
	}
	var user *domain.User
	var err error
	if strings.Contains(login, "@") {
		user, err = s.userRepo.FindByEmail(ctx, normalizeEmail(login))
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
func (s *AuthServiceImpl) Refresh(ctx context.Context, userID uint, issuedAt int64) (*dto.AuthResponseDTO, error) {
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
	d := toUserDTO(user)
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
				BranchID: b.ID, Name: b.Name, Slug: b.Slug, Status: b.Status(), Role: user.CompanyRole, Active: true,
			}
		}), nil
	}
	memberships, err := s.memberships.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapSlice(memberships, func(m domain.Membership) dto.MyBranchDTO {
		return dto.MyBranchDTO{
			BranchID: m.BranchID, Name: m.Branch.Name, Slug: m.Branch.Slug,
			Status: m.Branch.Status(), Role: m.Role, Active: m.Active,
		}
	}), nil
}

// ForgotPassword emails a one-hour reset link. Unknown or disabled accounts get no email,
// but the caller sees the same answer either way.
func (s *AuthServiceImpl) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.userRepo.FindByEmail(ctx, normalizeEmail(email))
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
func (s *AuthServiceImpl) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) error {
	if err := validPassword(req.Password); err != nil {
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

func issueTokens(user *domain.User) (*dto.AuthResponseDTO, error) {
	// role claim unused: roles are read from the database on every request
	pair, err := jwt.GenerateJWTToken(int(user.ID), 0)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponseDTO{
		// TokenPair holds the tokens as quoted JSON strings
		AccessToken:  strings.Trim(string(pair.AccessToken), `"`),
		RefreshToken: strings.Trim(string(pair.RefreshToken), `"`),
		User:         toUserDTO(user),
	}, nil
}

func toUserDTO(u *domain.User) dto.UserDTO {
	return dto.UserDTO{
		ID:             u.ID,
		Email:          u.Email,
		Avatar:         u.Avatar,
		Name:           u.Name,
		Username:       u.Username,
		Phone:          u.Phone,
		CompanyRole:    u.CompanyRole,
		Nationality:    u.Nationality,
		EmailGenerated: u.EmailGenerated,
		MemberNo:       domain.MemberNoOf(u.MemberNo),
		Status:         u.Status,
		CreatedAt:      u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      u.UpdatedAt.Format(time.RFC3339),
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isNotFound(err error) bool {
	var appErr response.AppError
	return errors.As(err, &appErr) && appErr.Status == http.StatusNotFound
}
