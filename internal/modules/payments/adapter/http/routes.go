package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	pollPerMinute  = 60   // one booking's page polls 20 a minute; this leaves room for two tabs and a refresh
	floodPerMinute = 1200 // all visitors of one IP together: 60 customers on one Wi-Fi, each polling 20 a minute

	startPerBooking = 10 // QRs one booking may ask for in startWindow
	startPerIP      = 60 // QRs one IP address may ask for in startWindow (a venue's Wi-Fi)
	startWindow     = 15 * time.Minute
)

// Routes: the customer pays and checks (by the booking's private link, or logged in), the gateway
// calls the webhook, and the team follows the money
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)
	manager := auth.Branch(domain.MemberManager)

	// The page asks about its payment every few seconds, so these routes are counted per booking (its private
	// token / the logged-in user), not per IP: a bar full of customers shares one Wi-Fi address. The loose
	// per-IP limit stays beside it as the flood guard (and slows anyone trying random tokens).
	flood := middleware.RateLimit(floodPerMinute, time.Minute)
	perBooking := middleware.RateLimitBy(pollPerMinute, time.Minute, func(c *fiber.Ctx) string { return "token:" + c.Params("token") })
	perUser := middleware.RateLimitBy(pollPerMinute, time.Minute, func(c *fiber.Ctx) string {
		return "user:" + strconv.FormatUint(uint64(middleware.CurrentUser(c).ID), 10)
	})

	// Making a QR spends a transaction at the gateway. One booking needs only a few (a bank tap, one more after a
	// QR ran out), so each booking (its token / the logged-in user) gets 10 in 15 minutes; the IP gets a looser 60
	// for a venue's Wi-Fi, and slows anyone trying random tokens.
	startIP := middleware.RateLimit(startPerIP, startWindow)
	startBooking := middleware.RateLimitBy(startPerBooking, startWindow, func(c *fiber.Ctx) string { return "token:" + c.Params("token") })
	startUser := middleware.RateLimitBy(startPerBooking, startWindow, func(c *fiber.Ctx) string {
		return "user:" + strconv.FormatUint(uint64(middleware.CurrentUser(c).ID), 10)
	})

	pub := api.Group("/public/branches/:slug")
	pub.Get("/payment-options", h.Options)
	pub.Post("/bookings/:token/pay", startIP, startBooking, h.StartByToken)
	pub.Get("/bookings/:token/payment", flood, perBooking, h.OpenByToken)
	pub.Get("/bookings/:token/payments/:id", flood, perBooking, h.CheckByToken)

	me := api.Group("/me/bookings", auth.RequireUser)
	me.Post("/:id/pay", startIP, startUser, h.StartByCustomer)
	me.Get("/:id/payment", flood, perUser, h.OpenByCustomer)
	me.Get("/:id/payments/:pid", flood, perUser, h.CheckByCustomer)

	// a customer's package (course pass): the same way to pay, by its own id
	pk := api.Group("/me/packages", auth.RequireUser)
	pk.Post("/:id/pay", startIP, startUser, h.StartPackage)
	pk.Get("/:id/payment", flood, perUser, h.OpenPackage)
	pk.Get("/:id/payments/:pid", flood, perUser, h.CheckPackage)

	// the gateway: no login, the signature is the proof
	api.Post("/payments/webhooks/:provider", h.Webhook)

	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Get("/bookings/:id/payments", staff, h.ForBooking)
	b.Get("/customer-packages/:id/payments", staff, h.ForPurchase)
	b.Get("/payments/attention", manager, h.Attention)
	b.Post("/payments/:id/refund", manager, h.Refund)
	// online payment on / off for the branch
	b.Get("/payment-settings", manager, h.Settings)
	b.Put("/payment-settings", manager, h.SaveSettings)
}
