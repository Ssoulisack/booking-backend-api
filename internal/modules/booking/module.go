// Package booking: how customers, guests and the branch team book and manage bookings — free times,
// booking a court, a coach or a class (and tables, for now), the staff desk, the calendar, status and
// payment marking. Hexagonal inside: the input ports, the service as use cases, and the HTTP adapter;
// the repositories are still the shared ones of the core (platform/postgres).
package booking

import (
	"backend/internal/modules/booking/adapter/http"
	"backend/internal/modules/booking/port"
	"backend/internal/modules/booking/service"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Deps are what the module is built from
type Deps = service.Deps

// Module is the booking module
type Module struct {
	// Service is the one service behind both the public page and the desk; other modules (events,
	// payments) use parts of it through their own ports, the company handler the desk part
	Service *service.Service
	handler *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	svc := service.New(d)
	return &Module{Service: svc, handler: http.New(svc, svc)}
}

// Desk is the staff desk's input port (the company handler lists customers through it)
func (m *Module) Desk() port.DeskService { return m.Service }

// Routes registers the module's public, customer and staff routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }
