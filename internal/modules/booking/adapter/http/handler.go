// Package http: the booking module's driving adapter: the public booking page, a customer's own
// bookings, and the branch team's booking desk.
package http

import (
	"backend/internal/core/dto"
	"backend/internal/modules/booking/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the booking routes (it only knows the input ports)
type Handler struct {
	book port.BookingService
	desk port.DeskService
	// perHour: new public bookings one IP address may make per hour (0: 10)
	perHour int
}

// WithPublicBookingsPerHour sets the hourly limit per IP address on the public booking route
func (h *Handler) WithPublicBookingsPerHour(n int) *Handler {
	h.perHour = n
	return h
}

// New builds the handler
func New(book port.BookingService, desk port.DeskService) *Handler {
	return &Handler{book: book, desk: desk}
}

// Availability expects ?date=YYYY-MM-DD and resource_id and/or coach_id (plus sport_id for a coach alone)
func (h *Handler) Availability(c *fiber.Ctx) error {
	var q dto.AvailabilityQuery
	if err := c.QueryParser(&q); err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.book.Availability(c.UserContext(), c.Params("slug"), q)
	return httpx.OK(c, res, err)
}

// CoachAvailability: ?date&resource_id&sport_id — the coaches free with the space that day, with their times
func (h *Handler) CoachAvailability(c *fiber.Ctx) error {
	var q dto.AvailabilityQuery
	if err := c.QueryParser(&q); err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.book.CoachAvailability(c.UserContext(), c.Params("slug"), q)
	return httpx.OK(c, res, err)
}

// Create books as the logged-in customer, or as a guest when no token is sent
func (h *Handler) Create(c *fiber.Ctx) error {
	var req dto.CreateBookingRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.Create(c.UserContext(), c.Params("slug"), middleware.CurrentUser(c), req)
	return httpx.Created(c, res, err)
}

func (h *Handler) FindBooking(c *fiber.Ctx) error {
	var req dto.FindBookingRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.FindBooking(c.UserContext(), c.Params("slug"), req)
	return httpx.OK(c, res, err)
}

func (h *Handler) FindCancel(c *fiber.Ctx) error {
	var req dto.FindBookingRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.FindCancel(c.UserContext(), c.Params("slug"), req)
	return httpx.OK(c, res, err)
}

func (h *Handler) GuestView(c *fiber.Ctx) error {
	res, err := h.book.GuestView(c.UserContext(), c.Params("slug"), c.Params("token"))
	return httpx.OK(c, res, err)
}

func (h *Handler) GuestCancel(c *fiber.Ctx) error {
	var req dto.CancelBookingRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.GuestCancel(c.UserContext(), c.Params("slug"), c.Params("token"), req)
	return httpx.OK(c, res, err)
}

func (h *Handler) MyBookings(c *fiber.Ctx) error {
	q, err := httpx.ListQuery(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.MyBookings(c.UserContext(), middleware.CurrentUser(c).ID, q)
	return httpx.Paged(c, res, err)
}

func (h *Handler) MyBooking(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.MyBooking(c.UserContext(), middleware.CurrentUser(c).ID, id)
	return httpx.OK(c, res, err)
}

// ClaimMine adds a booking made without an account to the logged-in customer's account
func (h *Handler) ClaimMine(c *fiber.Ctx) error {
	var req dto.ClaimBookingRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.ClaimMine(c.UserContext(), middleware.CurrentUser(c).ID, req)
	return httpx.OK(c, res, err)
}

func (h *Handler) CancelMine(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.CancelBookingRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.CancelMine(c.UserContext(), middleware.CurrentUser(c).ID, id, req)
	return httpx.OK(c, res, err)
}

// SlipSent: the customer says they paid and sent the slip (private link)
func (h *Handler) SlipSent(c *fiber.Ctx) error {
	slip, err := httpx.SlipUpload(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.SlipSent(c.UserContext(), c.Params("slug"), c.Params("token"), slip)
	return httpx.OK(c, res, err)
}

// MySlipSent: the same for a logged-in customer's own booking
func (h *Handler) MySlipSent(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	slip, err := httpx.SlipUpload(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.book.MySlipSent(c.UserContext(), middleware.CurrentUser(c).ID, id, slip)
	return httpx.OK(c, res, err)
}

func (h *Handler) ListBookings(c *fiber.Ctx) error {
	var q dto.BookingQuery
	if err := c.QueryParser(&q); err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.desk.ListBookings(c.UserContext(), middleware.BranchID(c), q)
	return httpx.Paged(c, res, err)
}

func (h *Handler) GetBooking(c *fiber.Ctx) error { return httpx.ScopedGet(c, h.desk.GetBooking) }

// Availability is the staff version of the public free times (same query: date + resource / coach)
func (h *Handler) DeskAvailability(c *fiber.Ctx) error {
	var q dto.AvailabilityQuery
	if err := c.QueryParser(&q); err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.desk.DeskAvailability(c.UserContext(), middleware.BranchID(c), q)
	return httpx.OK(c, res, err)
}

func (h *Handler) CreateBooking(c *fiber.Ctx) error {
	var req dto.StaffBookingRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.desk.CreateForCustomer(c.UserContext(), middleware.BranchID(c), middleware.CurrentUser(c), req)
	return httpx.Created(c, res, err)
}

func (h *Handler) ChangeStatus(c *fiber.Ctx) error { return httpx.ScopedUpdate(c, h.desk.ChangeStatus) }

func (h *Handler) SetPayment(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.PaymentRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.desk.SetPayment(c.UserContext(), middleware.BranchID(c), middleware.CurrentUser(c), id, req)
	return httpx.OK(c, res, err)
}

// Calendar expects ?date=YYYY-MM-DD and optionally &days=1..7
func (h *Handler) Calendar(c *fiber.Ctx) error {
	res, err := h.desk.Calendar(c.UserContext(), middleware.BranchID(c), c.Query("date"), c.QueryInt("days", 1))
	return httpx.OK(c, res, err)
}

func (h *Handler) Customers(c *fiber.Ctx) error { return httpx.ScopedList(c, h.desk.Customers) }

// FindCustomer expects ?q= a phone number, customer number (C0042) or email
func (h *Handler) FindCustomer(c *fiber.Ctx) error {
	res, err := h.desk.FindCustomer(c.UserContext(), middleware.BranchID(c), c.Query("q"))
	return httpx.OK(c, res, err)
}
