// Package identity: who people are and what they may do — accounts, login (password and Google /
// Facebook), password reset, the access check behind every protected route, and the invite links
// that bring team members in. Hexagonal inside: input ports, the services as use cases and the HTTP
// adapter; the repositories are still the shared ones of the core (platform/postgres).
package identity

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/identity/adapter/http"
	idpostgres "backend/internal/modules/identity/adapter/postgres"
	"backend/internal/modules/identity/domain"
	"backend/internal/modules/identity/port"
	"backend/internal/modules/identity/service"
	"backend/internal/platform/middleware"
	"backend/internal/platform/redis"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Deps are what the module is built from
type Deps struct {
	Users          output.UserRepository
	Cache          redis.CacheRepository
	Memberships    output.MembershipRepository
	Branches       output.BranchRepository
	Company        output.CompanyRepository
	PasswordResets output.PasswordResetRepository
	Social         output.SocialVerifier
	DB             *gorm.DB
	Mailer         output.Mailer
	FrontendURL    string
}

// Module is the identity module
type Module struct {
	Auth    port.AuthService
	Access  port.AccessService
	Invites port.InvitationService
	handler *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	m := &Module{
		Auth:    service.NewAuthService(d.Users, d.Cache, d.Memberships, d.Branches, d.PasswordResets, d.Social, idpostgres.NewSocialIdentities(d.DB), idpostgres.NewEmailCodes(d.DB), d.Mailer, d.FrontendURL),
		Access:  service.NewAccessService(d.Users, d.Memberships, d.Branches),
		Invites: service.NewInvitationService(idpostgres.NewInvitations(d.DB), d.Memberships, d.Branches, d.Company, d.Users, d.Mailer, d.FrontendURL),
	}
	m.handler = http.New(m.Auth, m.Invites)
	return m
}

// Middleware is the login / role check the other modules' routes use
func (m *Module) Middleware() *middleware.Auth { return middleware.NewAuth(m.Access) }

// Routes registers the module's routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }

// Models are the tables of this module, for the migration
func Models() []any {
	return []any{&domain.Invitation{}, &domain.SocialIdentity{}, &domain.EmailCode{}}
}
