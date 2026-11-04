// Package reports: summaries of a branch's bookings, rooms/courts use and money for a period, the
// daily cash-up, and the company-wide report. Read-only: it reads the shared repositories.
package reports

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/reports/adapter/http"
	"backend/internal/modules/reports/port"
	"backend/internal/modules/reports/service"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Deps are what the module is built from
type Deps struct {
	Branches  output.BranchRepository
	Bookings  output.BookingRepository
	Resources output.ResourceRepository
	Sports    output.SportRepository
	Hours     output.OpeningHourRepository
	Closures  output.ClosureRepository
}

// Module is the reports module
type Module struct {
	Service port.ReportService
	handler *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	svc := service.NewReportService(d.Branches, d.Bookings, d.Resources, d.Sports, d.Hours, d.Closures)
	return &Module{Service: svc, handler: http.New(svc)}
}

// Routes registers the module's routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }
