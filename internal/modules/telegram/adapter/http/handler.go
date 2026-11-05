// Package http: the Telegram settings routes. Owners and admins only: the page holds a bot's credentials.
package http

import (
	"backend/internal/core/domain"
	"backend/internal/modules/telegram/dto"
	"backend/internal/modules/telegram/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the module's routes
type Handler struct{ svc port.Service }

// New builds the handler
func New(svc port.Service) *Handler { return &Handler{svc: svc} }

// Routes registers a branch's Telegram settings (owner and admins)
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	admin := auth.Branch(domain.CompanyAdmin)
	b := api.Group("/branches/:branchId/telegram", auth.RequireUser)
	b.Get("", admin, h.Get)
	b.Put("", admin, h.Save)
	b.Delete("", admin, h.Disconnect)
	b.Post("/find-groups", admin, h.FindGroups)
	b.Post("/test", admin, h.Test)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	res, err := h.svc.Get(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) Save(c *fiber.Ctx) error {
	var req dto.SaveRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Save(c.UserContext(), middleware.BranchID(c), req)
	return httpx.OK(c, res, err)
}

func (h *Handler) FindGroups(c *fiber.Ctx) error {
	res, err := h.svc.FindGroups(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) Test(c *fiber.Ctx) error {
	res, err := h.svc.Test(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) Disconnect(c *fiber.Ctx) error {
	return httpx.Done(c, "TELEGRAM_DISCONNECTED", h.svc.Disconnect(c.UserContext(), middleware.BranchID(c)))
}
