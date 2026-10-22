package router

import (
	handler "backend/internal/adapters/primary/http/handler"
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/domain"

	"github.com/gofiber/fiber/v2"
)

// SetupManageRoutes registers one branch's dashboard routes under /branches/:branchId.
// Each route names the lowest role allowed: staff < manager < company (owner and admins, who
// act at every branch).
func SetupManageRoutes(
	router fiber.Router,
	auth *middleware.Auth,
	catalog handler.CatalogHandler,
	schedule handler.ScheduleHandler,
	rules handler.RuleHandler,
	media handler.MediaHandler,
	branch handler.BranchHandler,
	desk handler.DeskHandler,
	reports handler.ReportHandler,
) {
	staff := auth.Branch(domain.MemberStaff)
	manager := auth.Branch(domain.MemberManager)
	company := auth.Branch(domain.CompanyAdmin)

	b := router.Group("/branches/:branchId", auth.RequireUser)

	// the branch's own settings (company values in force are shown with them) and publishing
	b.Get("/settings", staff, branch.GetSettings)
	b.Put("/settings", company, branch.UpdateSettings)
	b.Post("/publish", company, branch.Publish)
	b.Post("/unpublish", company, branch.Unpublish)

	// catalog: everyone reads, managers change
	b.Get("/sports", staff, catalog.ListSports)
	b.Get("/sports/:id", staff, catalog.GetSport)
	b.Post("/sports", manager, catalog.CreateSport)
	b.Put("/sports/:id", manager, catalog.UpdateSport)
	b.Delete("/sports/:id", manager, catalog.DeleteSport)

	b.Get("/resources", staff, catalog.ListResources)
	b.Get("/resources/:id", staff, catalog.GetResource)
	b.Post("/resources", manager, catalog.CreateResource)
	b.Put("/resources/:id", manager, catalog.UpdateResource)
	b.Delete("/resources/:id", manager, catalog.DeleteResource)

	b.Get("/coaches", staff, catalog.ListCoaches)
	b.Get("/coaches/:id", staff, catalog.GetCoach)
	b.Post("/coaches", manager, catalog.CreateCoach)
	b.Put("/coaches/:id", manager, catalog.UpdateCoach)
	b.Delete("/coaches/:id", manager, catalog.DeleteCoach)

	// ownerType is "resource" or "coach"
	b.Get("/opening-hours/:ownerType/:ownerId", staff, schedule.GetOpeningHours)
	b.Put("/opening-hours/:ownerType/:ownerId", manager, schedule.SetOpeningHours)

	b.Get("/prices/:ownerType/:ownerId", staff, schedule.GetPrice)
	b.Put("/prices/:ownerType/:ownerId", manager, schedule.SetPrice)
	b.Delete("/prices/:ownerType/:ownerId", manager, schedule.DeletePrice)

	b.Get("/closures", staff, schedule.ListClosures)
	b.Get("/closures/:id", staff, schedule.GetClosure)
	b.Post("/closures", manager, schedule.CreateClosure)
	b.Put("/closures/:id", manager, schedule.UpdateClosure)
	b.Delete("/closures/:id", manager, schedule.DeleteClosure)

	b.Get("/class-sessions", staff, schedule.ListClassSessions)
	b.Get("/class-sessions/:id", staff, schedule.GetClassSession)
	b.Post("/class-sessions", manager, schedule.CreateClassSession)
	b.Post("/class-sessions/repeat", manager, schedule.RepeatClassSessions)
	b.Put("/class-sessions/:id", manager, schedule.UpdateClassSession)
	b.Delete("/class-sessions/:id", manager, schedule.DeleteClassSession)

	// scopeType is "branch" (id 0: this branch's changes to the company rule), "sport" or "resource"
	b.Get("/rules", staff, rules.ListRules)
	b.Get("/rules/effective", staff, rules.Effective)
	b.Get("/rules/:scopeType/:scopeId", staff, rules.GetRule)
	b.Put("/rules/:scopeType/:scopeId", manager, rules.SetRule)
	b.Delete("/rules/:scopeType/:scopeId", manager, rules.DeleteRule)

	b.Post("/uploads", manager, media.Upload)
	b.Delete("/uploads", manager, media.Delete)

	// booking desk: the whole team
	b.Get("/bookings", staff, desk.ListBookings)
	b.Get("/bookings/:id", staff, desk.GetBooking)
	b.Post("/bookings", staff, desk.CreateBooking)
	b.Get("/availability", staff, desk.Availability)
	b.Post("/bookings/:id/status", staff, desk.ChangeStatus)
	b.Post("/bookings/:id/payment", staff, desk.SetPayment)
	b.Get("/calendar", staff, desk.Calendar)
	b.Get("/customers", staff, desk.Customers)
	b.Get("/customers/find", staff, desk.FindCustomer)

	// reports and finance: managers and up; the daily cash-up also staff (to close the till)
	b.Get("/reports/cash-up", staff, reports.CashUp)
	b.Get("/reports/bookings", manager, reports.Bookings)
	b.Get("/reports/utilisation", manager, reports.Utilisation)
	b.Get("/reports/finance", manager, reports.Finance)
	b.Get("/reports/unpaid", manager, reports.Unpaid)
}
