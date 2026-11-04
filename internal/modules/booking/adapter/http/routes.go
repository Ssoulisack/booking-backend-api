package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Routes registers what the module serves: a branch's public booking page (customers and guests), a
// logged-in customer's own bookings, and the branch team's booking desk under /branches/:branchId
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)

	pub := api.Group("/public/branches/:slug")
	pub.Get("/availability", h.Availability)
	pub.Post("/bookings", middleware.RateLimit(1000, time.Hour), auth.OptionalUser, h.Create)
	// find a booking without its link (code + phone): POST keeps the phone out of addresses and logs;
	// few tries, so codes cannot be guessed
	pub.Post("/bookings/find", middleware.RateLimit(20, 15*time.Minute), h.FindBooking)
	pub.Post("/bookings/find/cancel", middleware.RateLimit(20, 15*time.Minute), h.FindCancel)
	pub.Get("/bookings/:token", h.GuestView)
	pub.Post("/bookings/:token/cancel", middleware.RateLimit(1000, 15*time.Minute), h.GuestCancel)
	pub.Post("/bookings/:token/slip-sent", middleware.RateLimit(60, 15*time.Minute), h.SlipSent)

	me := api.Group("/me/bookings", auth.RequireUser)
	me.Get("/", h.MyBookings)
	me.Get("/:id", h.MyBooking)
	me.Post("/:id/cancel", h.CancelMine)
	me.Post("/:id/slip-sent", h.MySlipSent)

	// booking desk: the whole team
	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Get("/bookings", staff, h.ListBookings)
	b.Get("/bookings/:id", staff, h.GetBooking)
	b.Post("/bookings", staff, h.CreateBooking)
	b.Get("/availability", staff, h.DeskAvailability)
	b.Post("/bookings/:id/status", staff, h.ChangeStatus)
	b.Post("/bookings/:id/payment", staff, h.SetPayment)
	b.Get("/calendar", staff, h.Calendar)
	b.Get("/customers", staff, h.Customers)
	b.Get("/customers/find", staff, h.FindCustomer)
}
