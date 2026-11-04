// Package catalog: what a branch offers and when — sports or areas, courts or tables with the floor
// plan, coaches, opening hours, closures, prices, group classes and weekly timetables — and the
// booking rules (company, branch, sport, court) with the merged "effective rule". The booking module
// reads the rules through the input port.
package catalog

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/catalog/adapter/http"
	"backend/internal/modules/catalog/port"
	"backend/internal/modules/catalog/service"
	"backend/internal/platform/middleware"
	"context"

	"github.com/gofiber/fiber/v2"
)

// Deps are what the module is built from
type Deps struct {
	Sports     output.SportRepository
	Resources  output.ResourceRepository
	Coaches    output.CoachRepository
	Classes    output.ClassSessionRepository
	Bookings   output.BookingRepository
	Branches   output.BranchRepository
	FloorItems output.FloorItemRepository
	Hours      output.OpeningHourRepository
	Closures   output.ClosureRepository
	Prices     output.PriceRepository
	Timetables output.ClassTimetableRepository
	Rules      output.RuleRepository
	Storage    output.FileStorage
	Media      output.MediaRepository
}

// Module is the catalog module
type Module struct {
	Catalog  port.CatalogService
	Schedule port.ScheduleService
	Rules    port.RuleService
	handler  *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	m := &Module{
		Catalog: service.NewCatalogService(d.Sports, d.Resources, d.Coaches, d.Classes, d.Bookings, d.Storage, d.Media, d.Branches, d.FloorItems),
		Schedule: service.NewScheduleService(d.Hours, d.Closures, d.Prices, d.Classes, d.Sports, d.Resources, d.Coaches, d.Bookings,
			d.Branches, d.Timetables),
		Rules: service.NewRuleService(d.Rules, d.Sports, d.Resources),
	}
	m.handler = http.New(m.Catalog, m.Schedule, m.Rules)
	return m
}

// RollTimetables makes the missing timetable classes once (the hourly job runs it)
func (m *Module) RollTimetables(ctx context.Context) error { return m.Schedule.RollTimetables(ctx) }

// Routes registers the module's routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }
