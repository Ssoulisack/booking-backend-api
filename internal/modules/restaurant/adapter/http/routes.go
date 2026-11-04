package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Routes registers the public floor plans and free tables, and the team's table desk
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)

	pub := api.Group("/public/branches/:slug")
	pub.Get("/floor-plans", h.FloorPlans)
	pub.Get("/table-availability", h.TableAvailability)

	// the group came in, other tables, a group without booking
	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Post("/bookings/:id/arrived", staff, h.MarkArrived)
	b.Post("/bookings/:id/move", staff, h.MoveTables)
	b.Post("/walk-ins", staff, h.WalkIn)
}
