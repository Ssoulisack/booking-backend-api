package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"

	"github.com/gofiber/fiber/v2"
)

type (
	// BranchHandler serves one branch's own settings and publishing
	BranchHandler interface {
		GetSettings(c *fiber.Ctx) error
		UpdateSettings(c *fiber.Ctx) error
		Publish(c *fiber.Ctx) error
		Unpublish(c *fiber.Ctx) error
	}
	branchHandler struct {
		svc input.BranchService
	}
)

func NewBranchHandler(svc input.BranchService) BranchHandler {
	return &branchHandler{svc: svc}
}

func (h *branchHandler) GetSettings(c *fiber.Ctx) error {
	res, err := h.svc.GetSettings(c.UserContext(), middleware.BranchID(c))
	return ok(c, res, err)
}

func (h *branchHandler) UpdateSettings(c *fiber.Ctx) error {
	var req dto.SettingsRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.UpdateSettings(c.UserContext(), middleware.BranchID(c), req)
	return ok(c, res, err)
}

func (h *branchHandler) Publish(c *fiber.Ctx) error {
	res, err := h.svc.Publish(c.UserContext(), middleware.BranchID(c))
	return ok(c, res, err)
}

func (h *branchHandler) Unpublish(c *fiber.Ctx) error {
	res, err := h.svc.Unpublish(c.UserContext(), middleware.BranchID(c))
	return ok(c, res, err)
}
