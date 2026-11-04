package router

import (
	handler "backend/internal/http/handler"
	"backend/internal/platform/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SetupBookingRoutes registers booking on a public page (customers and guests) and
// the logged-in customer's own bookings across every branch
func SetupBookingRoutes(router fiber.Router, auth *middleware.Auth, booking handler.BookingHandler) {
	pub := router.Group("/public/branches/:slug")
	pub.Get("/availability", booking.Availability)
	pub.Get("/floor-plans", booking.FloorPlans)
	pub.Get("/table-availability", booking.TableAvailability)
	pub.Post("/bookings", middleware.RateLimit(1000, time.Hour), auth.OptionalUser, booking.Create)
	// find a booking without its link (code + phone): POST keeps the phone out of addresses and logs;
	// few tries, so codes cannot be guessed
	pub.Post("/bookings/find", middleware.RateLimit(20, 15*time.Minute), booking.FindBooking)
	pub.Post("/bookings/find/cancel", middleware.RateLimit(20, 15*time.Minute), booking.FindCancel)
	pub.Get("/bookings/:token", booking.GuestView)
	pub.Post("/bookings/:token/cancel", middleware.RateLimit(1000, 15*time.Minute), booking.GuestCancel)

	me := router.Group("/me/bookings", auth.RequireUser)
	me.Get("/", booking.MyBookings)
	me.Get("/:id", booking.MyBooking)
	me.Post("/:id/cancel", booking.CancelMine)
}
