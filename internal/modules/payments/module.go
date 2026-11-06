// Package payments: online payment of bookings through a gateway. Hexagonal: domain and port in the
// middle, the service as its use cases, and adapters on both sides (HTTP in; PostgreSQL and the
// gateways, PhaJay first, out). This file builds them.
package payments

import (
	"backend/internal/core/port/output"
	paycache "backend/internal/modules/payments/adapter/cache"
	"backend/internal/modules/payments/adapter/http"
	"backend/internal/modules/payments/adapter/mock"
	"backend/internal/modules/payments/adapter/postgres"
	"backend/internal/modules/payments/domain"
	"backend/internal/modules/payments/port"
	"backend/internal/modules/payments/service"
	"backend/internal/platform/cache"
	"backend/internal/platform/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Deps are what the module is built from: the database for its own table, the gateways switched on,
// and the booking service
type Deps struct {
	DB        *gorm.DB
	Providers []port.Provider
	Branches  output.BranchRepository
	Bookings  port.Bookings
	// Purchases: the packages module (nil: course passes cannot be paid online)
	Purchases port.Purchases
	Now       func() time.Time
	// Cache keeps a branch's payment settings in Redis (nil or off: read from the database)
	Cache *cache.Cache
	// Dev is the in-process stand-in gateway (server.env: dev); its dev routes exist only then
	Dev *mock.Mock
}

// Module is the payments module: its service (for the job) and its routes
type Module struct {
	Service port.Service
	handler *http.Handler
	dev     *mock.Mock
}

// New builds the module
func New(d Deps) *Module {
	svc := service.New(service.Deps{
		Repo: paycache.Wrap(postgres.New(d.DB), d.Cache), Providers: d.Providers, Branches: d.Branches, Bookings: d.Bookings, Purchases: d.Purchases, Now: d.Now,
	})
	return &Module{Service: svc, handler: http.New(svc), dev: d.Dev}
}

// Routes registers the module's customer, webhook and staff routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) {
	m.handler.Routes(api, auth)
	if m.dev != nil {
		m.dev.Routes(api, m.Service)
	}
}

// Models are the tables of this module, for the migration
func Models() []any { return []any{&domain.Payment{}, &domain.Setting{}} }
