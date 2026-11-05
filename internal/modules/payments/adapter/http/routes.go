package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Routes: the customer pays and checks (by the booking's private link, or logged in), the gateway
// calls the webhook, and the team follows the money
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)
	manager := auth.Branch(domain.MemberManager)

	pub := api.Group("/public/branches/:slug")
	pub.Get("/payment-options", h.Options)
	pub.Post("/bookings/:token/pay", middleware.RateLimit(60, 15*time.Minute), h.StartByToken)
	pub.Get("/bookings/:token/payment", middleware.RateLimit(240, 15*time.Minute), h.OpenByToken)
	pub.Get("/bookings/:token/payments/:id", middleware.RateLimit(240, 15*time.Minute), h.CheckByToken)

	me := api.Group("/me/bookings", auth.RequireUser)
	me.Post("/:id/pay", middleware.RateLimit(60, 15*time.Minute), h.StartByCustomer)
	me.Get("/:id/payment", middleware.RateLimit(240, 15*time.Minute), h.OpenByCustomer)
	me.Get("/:id/payments/:pid", middleware.RateLimit(240, 15*time.Minute), h.CheckByCustomer)

	// the gateway: no login, the signature is the proof
	api.Post("/payments/webhooks/:provider", h.Webhook)

	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Get("/bookings/:id/payments", staff, h.ForBooking)
	b.Get("/payments/attention", manager, h.Attention)
	b.Post("/payments/:id/refund", manager, h.Refund)
	// online payment on / off for the branch
	b.Get("/payment-settings", manager, h.Settings)
	b.Put("/payment-settings", manager, h.SaveSettings)
}
