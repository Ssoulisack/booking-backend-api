// Package events: event days at a restaurant (see domain). The module is hexagonal: domain and port
// in the middle, the service as its use cases, and adapters on both sides (HTTP in, PostgreSQL out);
// this file builds them.
package events

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/events/adapter/http"
	"backend/internal/modules/events/adapter/postgres"
	"backend/internal/modules/events/domain"
	"backend/internal/modules/events/port"
	"backend/internal/modules/events/service"
	"backend/internal/platform/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Deps are what the module is built from: the database for its own tables, and the other parts of the
// app it talks to
type Deps struct {
	DB         *gorm.DB
	Branches   output.BranchRepository
	Sports     output.SportRepository
	Resources  output.ResourceRepository
	FloorItems output.FloorItemRepository
	Bookings   output.BookingRepository
	Media      output.MediaRepository
	Files      output.FileStorage
	Booker     port.Booker
	// Notifier emails the customer when their unpaid booking is released (nil: no email)
	Notifier port.Notifier
	Now      func() time.Time
}

// Module is the events module: its service (for the booking service and the job) and its routes
type Module struct {
	Service port.Service
	handler *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	svc := service.New(service.Deps{
		Repo: postgres.New(d.DB), Branches: d.Branches, Sports: d.Sports, Resources: d.Resources,
		FloorItems: d.FloorItems, Bookings: d.Bookings, Media: d.Media, Files: d.Files, Booker: d.Booker, Notifier: d.Notifier, Now: d.Now,
	})
	return &Module{Service: svc, handler: http.New(svc)}
}

// Routes registers the module's staff and public routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }

// Models are the tables of this module, for the migration
func Models() []any { return []any{&domain.Event{}, &domain.Price{}, &domain.PayInfo{}} }
