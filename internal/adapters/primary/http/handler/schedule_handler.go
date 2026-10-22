package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"

	"github.com/gofiber/fiber/v2"
)

type (
	ScheduleHandler interface {
		GetOpeningHours(c *fiber.Ctx) error
		SetOpeningHours(c *fiber.Ctx) error

		CreateClosure(c *fiber.Ctx) error
		GetClosure(c *fiber.Ctx) error
		ListClosures(c *fiber.Ctx) error
		UpdateClosure(c *fiber.Ctx) error
		DeleteClosure(c *fiber.Ctx) error

		GetPrice(c *fiber.Ctx) error
		SetPrice(c *fiber.Ctx) error
		DeletePrice(c *fiber.Ctx) error

		CreateClassSession(c *fiber.Ctx) error
		GetClassSession(c *fiber.Ctx) error
		ListClassSessions(c *fiber.Ctx) error
		UpdateClassSession(c *fiber.Ctx) error
		DeleteClassSession(c *fiber.Ctx) error
		RepeatClassSessions(c *fiber.Ctx) error
	}
	scheduleHandler struct {
		svc input.ScheduleService
	}
)

func NewScheduleHandler(svc input.ScheduleService) ScheduleHandler {
	return &scheduleHandler{svc: svc}
}

func (h *scheduleHandler) GetOpeningHours(c *fiber.Ctx) error {
	ownerType, ownerID, err := ownerParams(c)
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.GetOpeningHours(c.UserContext(), middleware.BranchID(c), ownerType, ownerID)
	return ok(c, res, err)
}

func (h *scheduleHandler) SetOpeningHours(c *fiber.Ctx) error {
	ownerType, ownerID, err := ownerParams(c)
	if err != nil {
		return fail(c, err)
	}
	var req dto.OpeningHoursRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.SetOpeningHours(c.UserContext(), middleware.BranchID(c), ownerType, ownerID, req)
	return ok(c, res, err)
}

func (h *scheduleHandler) CreateClosure(c *fiber.Ctx) error {
	return scopedCreate(c, h.svc.CreateClosure)
}
func (h *scheduleHandler) GetClosure(c *fiber.Ctx) error   { return scopedGet(c, h.svc.GetClosure) }
func (h *scheduleHandler) ListClosures(c *fiber.Ctx) error { return scopedList(c, h.svc.ListClosures) }
func (h *scheduleHandler) UpdateClosure(c *fiber.Ctx) error {
	return scopedUpdate(c, h.svc.UpdateClosure)
}
func (h *scheduleHandler) DeleteClosure(c *fiber.Ctx) error {
	return scopedRemove(c, h.svc.DeleteClosure)
}

func (h *scheduleHandler) GetPrice(c *fiber.Ctx) error {
	ownerType, ownerID, err := ownerParams(c)
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.GetPrice(c.UserContext(), middleware.BranchID(c), ownerType, ownerID)
	return ok(c, res, err)
}

func (h *scheduleHandler) SetPrice(c *fiber.Ctx) error {
	ownerType, ownerID, err := ownerParams(c)
	if err != nil {
		return fail(c, err)
	}
	var req dto.PriceRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.SetPrice(c.UserContext(), middleware.BranchID(c), ownerType, ownerID, req)
	return ok(c, res, err)
}

func (h *scheduleHandler) DeletePrice(c *fiber.Ctx) error {
	ownerType, ownerID, err := ownerParams(c)
	if err != nil {
		return fail(c, err)
	}
	return done(c, "DELETED", h.svc.DeletePrice(c.UserContext(), middleware.BranchID(c), ownerType, ownerID))
}

func (h *scheduleHandler) CreateClassSession(c *fiber.Ctx) error {
	return scopedCreate(c, h.svc.CreateClassSession)
}
func (h *scheduleHandler) GetClassSession(c *fiber.Ctx) error {
	return scopedGet(c, h.svc.GetClassSession)
}
func (h *scheduleHandler) ListClassSessions(c *fiber.Ctx) error {
	return scopedList(c, h.svc.ListClassSessions)
}
func (h *scheduleHandler) UpdateClassSession(c *fiber.Ctx) error {
	return scopedUpdate(c, h.svc.UpdateClassSession)
}
func (h *scheduleHandler) DeleteClassSession(c *fiber.Ctx) error {
	return scopedRemove(c, h.svc.DeleteClassSession)
}

// RepeatClassSessions creates a class on every chosen weekday between two dates
func (h *scheduleHandler) RepeatClassSessions(c *fiber.Ctx) error {
	return scopedCreate(c, h.svc.RepeatClassSessions)
}
