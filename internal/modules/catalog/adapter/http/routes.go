package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Routes registers one branch's catalog, schedule and rules under /branches/:branchId, and the
// company's default booking rule. Each route names the lowest role allowed: staff < manager < company.
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)
	manager := auth.Branch(domain.MemberManager)

	b := api.Group("/branches/:branchId", auth.RequireUser)

	// catalog: everyone reads, managers change
	b.Get("/sports", staff, h.ListSports)
	b.Get("/sports/:id", staff, h.GetSport)
	b.Post("/sports", manager, h.CreateSport)
	b.Put("/sports/:id", manager, h.UpdateSport)
	b.Delete("/sports/:id", manager, h.DeleteSport)
	// restaurant areas: the floor plan (size and where each table stands), saved at once
	b.Get("/sports/:id/plan", staff, h.GetFloorPlan)
	b.Put("/sports/:id/plan", manager, h.SaveFloorPlan)

	b.Get("/resources", staff, h.ListResources)
	b.Get("/resources/:id", staff, h.GetResource)
	b.Post("/resources", manager, h.CreateResource)
	b.Put("/resources/:id", manager, h.UpdateResource)
	b.Delete("/resources/:id", manager, h.DeleteResource)

	b.Get("/coaches", staff, h.ListCoaches)
	b.Get("/coaches/:id", staff, h.GetCoach)
	b.Post("/coaches", manager, h.CreateCoach)
	b.Put("/coaches/:id", manager, h.UpdateCoach)
	b.Delete("/coaches/:id", manager, h.DeleteCoach)

	// ownerType is "resource" or "coach"
	b.Get("/opening-hours/:ownerType/:ownerId", staff, h.GetOpeningHours)
	b.Put("/opening-hours/:ownerType/:ownerId", manager, h.SetOpeningHours)

	b.Get("/prices/:ownerType/:ownerId", staff, h.GetPrice)
	b.Put("/prices/:ownerType/:ownerId", manager, h.SetPrice)
	b.Delete("/prices/:ownerType/:ownerId", manager, h.DeletePrice)

	b.Get("/closures", staff, h.ListClosures)
	b.Get("/closures/:id", staff, h.GetClosure)
	b.Post("/closures", manager, h.CreateClosure)
	b.Put("/closures/:id", manager, h.UpdateClosure)
	b.Delete("/closures/:id", manager, h.DeleteClosure)

	b.Get("/class-sessions", staff, h.ListClassSessions)
	b.Get("/class-sessions/:id", staff, h.GetClassSession)
	b.Post("/class-sessions", manager, h.CreateClassSession)
	b.Post("/class-sessions/repeat", manager, h.RepeatClassSessions)
	b.Put("/class-sessions/:id", manager, h.UpdateClassSession)
	b.Delete("/class-sessions/:id", manager, h.DeleteClassSession)

	// weekly timetables: classes are made automatically for the next 30 days
	b.Get("/class-timetables", staff, h.ListTimetables)
	b.Post("/class-timetables", manager, h.CreateTimetable)
	b.Put("/class-timetables/:id", manager, h.UpdateTimetable)
	b.Delete("/class-timetables/:id", manager, h.DeleteTimetable)

	// scopeType is "branch" (id 0: this branch's changes to the company rule), "sport" or "resource"
	b.Get("/rules", staff, h.ListRules)
	b.Get("/rules/effective", staff, h.Effective)
	b.Get("/rules/company", staff, h.CompanyRule) // what an empty branch field means
	b.Get("/rules/:scopeType/:scopeId", staff, h.GetRule)
	b.Put("/rules/:scopeType/:scopeId", manager, h.SetRule)
	b.Delete("/rules/:scopeType/:scopeId", manager, h.DeleteRule)

	// the company default rule: every branch starts from it
	co := api.Group("/company/rules", auth.RequireUser, auth.RequireCompany)
	co.Get("/", h.GetCompanyRule)
	co.Put("/", h.SetCompanyRule)
}
