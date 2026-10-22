package router

import (
	handler "backend/internal/adapters/primary/http/handler"
	"backend/internal/adapters/primary/http/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SetupBookingRoutes registers booking on a public page (customers and guests) and
// the logged-in customer's own bookings across every branch
func SetupBookingRoutes(router fiber.Router, auth *middleware.Auth, booking handler.BookingHandler) {
	pub := router.Group("/public/branches/:slug")
	pub.Get("/availability", booking.Availability)
	pub.Post("/bookings", middleware.RateLimit(60, time.Hour), auth.OptionalUser, booking.Create)
	pub.Get("/bookings/:token", booking.GuestView)
	pub.Post("/bookings/:token/cancel", middleware.RateLimit(10, 15*time.Minute), booking.GuestCancel)

	me := router.Group("/me/bookings", auth.RequireUser)
	me.Get("/", booking.MyBookings)
	me.Get("/:id", booking.MyBooking)
	me.Post("/:id/cancel", booking.CancelMine)
}
