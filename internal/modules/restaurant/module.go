// Package restaurant: booking tables — the floor plans and free tables customers see, the table
// booking itself (also for event days), and the team's table desk: arrived, move to other tables,
// walk-ins, and the job that settles late groups and finished evenings. It borrows the booking row,
// opening hours and "who is booking" from the booking module (its Kernel); the booking module hands
// restaurant branches to it.
package restaurant

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/restaurant/adapter/http"
	"backend/internal/modules/restaurant/service"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Deps are what the module is built from
type Deps = service.Deps

// Module is the restaurant module
type Module struct {
	Service *service.Service
	handler *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	svc := service.New(d)
	return &Module{Service: svc, handler: http.New(svc, svc)}
}

// SetTeamNotifier lets the module tell the team (Telegram) about a group that did not come in time
func (m *Module) SetTeamNotifier(n output.TeamNotifier) { m.Service.SetTeamNotifier(n) }

// SetEventDays tells the module how to ask whether an event takes over a day
func (m *Module) SetEventDays(e output.EventDays) { m.Service.SetEventDays(e) }

// Routes registers the module's routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }
