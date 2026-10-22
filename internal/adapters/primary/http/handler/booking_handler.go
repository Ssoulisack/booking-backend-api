package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type (
	BookingHandler interface {
		Availability(c *fiber.Ctx) error
		Create(c *fiber.Ctx) error
		GuestView(c *fiber.Ctx) error
		GuestCancel(c *fiber.Ctx) error
		MyBookings(c *fiber.Ctx) error
		MyBooking(c *fiber.Ctx) error
		CancelMine(c *fiber.Ctx) error
	}
	bookingHandler struct {
		svc input.BookingService
	}
)

func NewBookingHandler(svc input.BookingService) BookingHandler {
	return &bookingHandler{svc: svc}
}

// Availability expects ?date=YYYY-MM-DD and resource_id and/or coach_id (plus sport_id for a coach alone)
func (h *bookingHandler) Availability(c *fiber.Ctx) error {
	var q dto.AvailabilityQuery
	if err := c.QueryParser(&q); err != nil {
		return fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.svc.Availability(c.UserContext(), c.Params("slug"), q)
	return ok(c, res, err)
}

// Create books as the logged-in customer, or as a guest when no token is sent
func (h *bookingHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateBookingRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.Create(c.UserContext(), c.Params("slug"), middleware.CurrentUser(c), req)
	return created(c, res, err)
}

func (h *bookingHandler) GuestView(c *fiber.Ctx) error {
	res, err := h.svc.GuestView(c.UserContext(), c.Params("slug"), c.Params("token"))
	return ok(c, res, err)
}

func (h *bookingHandler) GuestCancel(c *fiber.Ctx) error {
	var req dto.CancelBookingRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.GuestCancel(c.UserContext(), c.Params("slug"), c.Params("token"), req)
	return ok(c, res, err)
}

func (h *bookingHandler) MyBookings(c *fiber.Ctx) error {
	q, err := listQuery(c)
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.MyBookings(c.UserContext(), middleware.CurrentUser(c).ID, q)
	return paged(c, res, err)
}

func (h *bookingHandler) MyBooking(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.MyBooking(c.UserContext(), middleware.CurrentUser(c).ID, id)
	return ok(c, res, err)
}

func (h *bookingHandler) CancelMine(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req dto.CancelBookingRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.CancelMine(c.UserContext(), middleware.CurrentUser(c).ID, id, req)
	return ok(c, res, err)
}
