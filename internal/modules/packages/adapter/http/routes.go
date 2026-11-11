package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Routes registers the staff routes (under /branches/:branchId: the team reads, managers change) and
// the public list of what a branch sells
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)
	manager := auth.Branch(domain.MemberManager)

	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Get("/packages", staff, h.List)
	b.Post("/packages", manager, h.Create)
	b.Get("/packages/:id", staff, h.Get)
	b.Put("/packages/:id", manager, h.Update)
	b.Delete("/packages/:id", manager, h.Delete)

	// packages customers own: the team sees them, sells at the desk, marks paid; managers cancel
	b.Get("/customer-packages", staff, h.Sold)
	b.Post("/customer-packages", staff, h.Sell)
	b.Get("/members", staff, h.Members) // customers who bought a package (the Membership tab)
	b.Post("/customer-packages/:id/paid", staff, h.MarkPaid)
	b.Post("/customer-packages/:id/cancel", manager, h.Cancel)
	b.Get("/customer-packages/:id/visits", staff, h.Visits)
	b.Post("/customer-packages/:id/adjust", manager, h.Adjust)

	api.Get("/public/branches/:slug/packages", h.Public)
	// a signed-in customer buys (it waits for payment), sees theirs, drops one not paid yet
	api.Post("/public/branches/:slug/packages/:id/buy", middleware.RateLimit(30, time.Hour), auth.RequireUser, h.Buy)
	me := api.Group("/me/packages", auth.RequireUser)
	me.Get("/", h.Mine)
	me.Post("/:id/cancel", h.CancelMine)
	me.Get("/:id/visits", h.MyVisits)
	me.Post("/:id/slip", middleware.RateLimit(20, time.Hour), h.SlipSent)
}
