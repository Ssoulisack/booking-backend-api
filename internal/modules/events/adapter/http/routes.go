package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Routes registers the staff routes (under /branches/:branchId: the team reads, managers change) and
// the public ones (the ads, an event day, buying tables)
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)
	manager := auth.Branch(domain.MemberManager)

	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Get("/events", staff, h.List)
	b.Post("/events", manager, h.Create)
	b.Get("/events/:id", staff, h.Get)
	b.Put("/events/:id", manager, h.Update)
	b.Delete("/events/:id", manager, h.Delete)
	b.Post("/events/:id/copy", manager, h.Copy)
	b.Get("/events/:id/prices", staff, h.Prices)
	b.Put("/events/:id/prices", manager, h.SetPrices)
	b.Get("/event-pay", staff, h.Pay)
	b.Put("/event-pay", manager, h.SavePay)

	pub := api.Group("/public/branches/:slug")
	pub.Get("/events", h.PublicList)
	pub.Get("/events/:id", h.PublicDetail)
	pub.Get("/event-pay", h.PublicPay)
	pub.Post("/events/:id/bookings", middleware.RateLimit(1000, time.Hour), auth.OptionalUser, h.PublicBook)
}
