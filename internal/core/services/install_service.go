package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"backend/pkg/utils"
	"context"
	"log"
	"net/http"
	"strings"
	"time"
)

// welcomeTTL is how long the first owner's "choose your password" link works
const welcomeTTL = 7 * 24 * time.Hour

// InstallConfig is the company block of config.yaml
type InstallConfig struct {
	CompanyName   string
	OwnerName     string
	OwnerEmail    string
	OwnerPassword string
}

type InstallServiceImpl struct {
	company     output.CompanyRepository
	users       output.UserRepository
	rules       output.RuleRepository
	resets      output.PasswordResetRepository
	mailer      output.Mailer
	frontendURL string
	cfg         InstallConfig
	now         func() time.Time
}

func NewInstallService(
	company output.CompanyRepository,
	users output.UserRepository,
	rules output.RuleRepository,
	resets output.PasswordResetRepository,
	mailer output.Mailer,
	frontendURL string,
	cfg InstallConfig,
) input.InstallService {
	return &InstallServiceImpl{
		company: company, users: users, rules: rules, resets: resets, mailer: mailer,
		frontendURL: strings.TrimRight(frontendURL, "/"), cfg: cfg, now: time.Now,
	}
}

// EnsureInstalled creates what a fresh install needs; anything that exists is left alone
func (s *InstallServiceImpl) EnsureInstalled(ctx context.Context) error {
	company, err := s.ensureCompany(ctx)
	if err != nil {
		return err
	}
	if err := s.ensureCompanyRule(ctx); err != nil {
		return err
	}
	return s.ensureOwner(ctx, company)
}

func (s *InstallServiceImpl) ensureCompany(ctx context.Context) (*domain.Company, error) {
	c, err := s.company.Get(ctx)
	if err == nil {
		return c, nil
	}
	if !isNotFound(err) {
		return nil, err
	}
	name := strings.TrimSpace(s.cfg.CompanyName)
	if name == "" {
		return nil, response.NewError(http.StatusInternalServerError, "company.name is missing in config.yaml")
	}
	c = &domain.Company{
		Name: name, Language: "en", Currency: "LAK", Timezone: "Asia/Vientiane",
		BookingMode: domain.BookingModeAccount,
	}
	if err := s.company.Create(ctx, c); err != nil {
		return nil, err
	}
	log.Printf("🏢 company created: %s", name)
	return c, nil
}

func (s *InstallServiceImpl) ensureCompanyRule(ctx context.Context) error {
	r, err := s.rules.GetByScope(ctx, 0, domain.ScopeCompany, 0)
	if err != nil || r != nil {
		return err
	}
	rule := domain.DefaultCompanyRule()
	return s.rules.Upsert(ctx, &rule)
}

// ensureOwner makes the configured person the owner when nobody is yet: an existing account is
// promoted, otherwise one is created. Without a configured password the owner gets a welcome
// email with a link to choose one.
func (s *InstallServiceImpl) ensureOwner(ctx context.Context, c *domain.Company) error {
	_, total, err := s.users.List(ctx, output.ListQuery{Limit: 1, Filters: map[string]any{"company_role": domain.CompanyOwner}})
	if err != nil {
		return err
	}
	if total > 0 {
		return nil
	}
	email := normalizeEmail(s.cfg.OwnerEmail)
	if !validEmail(email) {
		return response.NewError(http.StatusInternalServerError, "company.ownerEmail is missing or invalid in config.yaml")
	}
	name := strings.TrimSpace(s.cfg.OwnerName)
	if name == "" {
		name = "Owner"
	}

	user, err := s.users.FindByEmail(ctx, email)
	switch {
	case err == nil:
		if err := s.users.SetCompanyRole(ctx, user.ID, domain.CompanyOwner); err != nil {
			return err
		}
		log.Printf("👑 owner: existing account %s", email)
		return nil
	case !isNotFound(err):
		return err
	}

	password := s.cfg.OwnerPassword
	if password == "" {
		raw, _, err := newToken() // an unknown password; the owner chooses one through the welcome link
		if err != nil {
			return err
		}
		password = raw
	} else if err := validPassword(password); err != nil {
		return err
	}
	hash, err := utils.Encrypt(password)
	if err != nil {
		return err
	}
	user = &domain.User{
		Email: email, Username: email, Name: name, PasswordHash: hash, CompanyRole: domain.CompanyOwner,
		Verified: true, Status: true, LastActive: s.now(),
	}
	if err := s.users.Create(ctx, user); err != nil {
		return err
	}
	if s.cfg.OwnerPassword != "" {
		log.Printf("👑 owner created: %s (password from config.yaml)", email)
		return nil
	}
	raw, hash2, err := newToken()
	if err != nil {
		return err
	}
	if err := s.resets.Create(ctx, &domain.PasswordReset{UserID: user.ID, TokenHash: hash2, ExpiresAt: s.now().Add(welcomeTTL)}); err != nil {
		return err
	}
	link := s.frontendURL + "/reset-password/" + raw
	if err := s.mailer.SendWelcome(email, name, c.Name, link); err != nil {
		log.Printf("❌ welcome email to %s failed: %v — use Forgot password on the login page", email, err)
		return nil
	}
	log.Printf("👑 owner created: %s — welcome email sent", email)
	return nil
}
