// Package company: the one company of this install — its brand and settings, branches (create, close,
// reopen, publish), team and users, the first-start install, one branch's own settings, and the
// public site (brand, list of places, a place's sports, courts, coaches and classes).
package company

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/company/adapter/http"
	"backend/internal/modules/company/port"
	"backend/internal/modules/company/service"
	"backend/internal/platform/middleware"
	"context"

	"github.com/gofiber/fiber/v2"
)

// InstallConfig is the company block of config.yaml
type InstallConfig = service.InstallConfig

// Customers lists the accounts who booked at any branch (the booking module's desk)
type Customers = http.Customers

// Deps are what the module is built from
type Deps struct {
	Company        output.CompanyRepository
	Branches       output.BranchRepository
	Directory      output.DirectoryRepository
	Memberships    output.MembershipRepository
	Users          output.UserRepository
	Sports         output.SportRepository
	Resources      output.ResourceRepository
	Coaches        output.CoachRepository
	Classes        output.ClassSessionRepository
	Bookings       output.BookingRepository
	Prices         output.PriceRepository
	OpeningHours   output.OpeningHourRepository
	Rules          output.RuleRepository
	PasswordResets output.PasswordResetRepository
	Storage        output.FileStorage
	Media          output.MediaRepository
	Mailer         output.Mailer
	FrontendURL    string
	Install        InstallConfig
	Customers      Customers
}

// Module is the company module
type Module struct {
	Company port.CompanyService
	Branch  port.BranchService
	Public  port.PublicService
	install port.InstallService
	handler *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	m := &Module{
		Company: service.NewCompanyService(d.Company, d.Branches, d.Memberships, d.Users, d.Storage, d.Media, d.Mailer, d.FrontendURL),
		Branch:  service.NewBranchService(d.Branches, d.Sports, d.Resources, d.OpeningHours, d.Prices, d.Storage, d.Media),
		Public: service.NewPublicService(d.Company, d.Branches, d.Directory, d.Sports, d.Resources, d.Coaches,
			d.Classes, d.Bookings, d.Prices, d.OpeningHours, d.Storage),
		install: service.NewInstallService(d.Company, d.Users, d.Rules, d.PasswordResets, d.Mailer, d.FrontendURL, d.Install),
	}
	m.handler = http.New(m.Company, m.Branch, m.Public, d.Customers)
	return m
}

// EnsureInstalled prepares a fresh install (company, its booking rule, the first owner); safe on every start
func (m *Module) EnsureInstalled(ctx context.Context) error { return m.install.EnsureInstalled(ctx) }

// Routes registers the module's routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }
