package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Routes registers a branch's reports (managers and up; the daily cash-up also staff, to close the
// till) and the company-wide report (owner and admins)
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)
	manager := auth.Branch(domain.MemberManager)

	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Get("/reports/cash-up", staff, h.CashUp)
	b.Get("/reports/bookings", manager, h.Bookings)
	b.Get("/reports/utilisation", manager, h.Utilisation)
	b.Get("/reports/finance", manager, h.Finance)
	b.Get("/reports/unpaid", manager, h.Unpaid)
	b.Get("/reports/table-bookings", manager, h.TableBookings)
	b.Get("/reports/event-bookings", manager, h.EventBookings)

	api.Get("/company/report", auth.RequireUser, auth.RequireCompany, h.Company)
}
