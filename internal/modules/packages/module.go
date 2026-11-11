// Package packages: course passes a sport branch sells (10 / 20 visits, 1 / 3 month passes) for group
// classes or a space. Hexagonal like the other modules: domain and port in the middle, the service as its
// use cases, adapters on both sides (HTTP in, PostgreSQL out); this file builds them.
package packages

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/packages/adapter/http"
	"backend/internal/modules/packages/adapter/postgres"
	"backend/internal/modules/packages/domain"
	"backend/internal/modules/packages/port"
	"backend/internal/modules/packages/service"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Deps are what the module is built from
type Deps struct {
	DB         *gorm.DB
	Branches   output.BranchRepository
	Sports     output.SportRepository
	Resources  output.ResourceRepository
	Timetables output.ClassTimetableRepository
	Coaches    output.CoachRepository
	Users      output.UserRepository
}

// Module is the packages module
type Module struct {
	Service port.Service
	svc     *service.Service
	handler *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	svc := service.New(service.Deps{Repo: postgres.New(d.DB), Branches: d.Branches, Sports: d.Sports, Resources: d.Resources, Timetables: d.Timetables, Coaches: d.Coaches, Users: d.Users})
	return &Module{Service: svc, svc: svc, handler: http.New(svc)}
}

// SetNews gives the module a way to tell the branch's team about package sales (nil: nobody)
func (m *Module) SetNews(n output.TeamNews) { m.svc.SetNews(n) }

// SetSlips tells the module how to read a branch's slip ways and where uploaded slips go (sender nil: no Telegram)
func (m *Module) SetSlips(w port.SlipWays, sender port.SlipSender) { m.svc.SetSlips(w, sender) }

// Routes registers the module's staff and public routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }

// Models are the tables of this module, for the migration
func Models() []any {
	return []any{&domain.Package{}, &domain.Option{}, &domain.CustomerPackage{}, &domain.Visit{}}
}
