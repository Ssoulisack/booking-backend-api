package http

import (
	"backend/internal/modules/payments/domain"
	"backend/internal/modules/payments/dto"
	"backend/internal/modules/payments/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"
	"errors"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the module's routes (the driving side: it only knows the input port)
type Handler struct{ svc port.Service }

// New builds the handler
func New(svc port.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Options(c *fiber.Ctx) error {
	res, err := h.svc.Options(c.UserContext(), c.Params("slug"), c.Query("for") == "package")
	return httpx.OK(c, res, err)
}

func (h *Handler) StartByToken(c *fiber.Ctx) error {
	var req dto.StartRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.StartByToken(c.UserContext(), c.Params("slug"), c.Params("token"), req)
	return httpx.Created(c, res, err)
}

func (h *Handler) CheckByToken(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.CheckByToken(c.UserContext(), c.Params("slug"), c.Params("token"), id)
	return httpx.OK(c, res, err)
}

func (h *Handler) StartByCustomer(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.StartRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.StartByCustomer(c.UserContext(), middleware.CurrentUser(c).ID, id, req)
	return httpx.Created(c, res, err)
}

func (h *Handler) CheckByCustomer(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	pid, err := httpx.ID(c, "pid")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.CheckByCustomer(c.UserContext(), middleware.CurrentUser(c).ID, id, pid)
	return httpx.OK(c, res, err)
}

// Webhook is the gateway's callback: 200 when signed (and applied or ignored), 401 when not signed
func (h *Handler) Webhook(c *fiber.Ctx) error {
	err := h.svc.HandleWebhook(c.UserContext(), c.Params("provider"), func(k string) string { return c.Get(k) }, c.Body())
	switch {
	case err == nil:
		return c.SendStatus(fiber.StatusOK)
	case errors.Is(err, domain.ErrBadSignature):
		return c.SendStatus(fiber.StatusUnauthorized)
	case errors.Is(err, domain.ErrNotFound):
		return c.SendStatus(fiber.StatusNotFound)
	default:
		return c.SendStatus(fiber.StatusInternalServerError) // the gateway may send it again
	}
}

func (h *Handler) ForBooking(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.ForBooking(c.UserContext(), middleware.BranchID(c), id)
	return httpx.OK(c, res, err)
}

func (h *Handler) Attention(c *fiber.Ctx) error {
	res, err := h.svc.Attention(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) Refund(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Refund(c.UserContext(), middleware.BranchID(c), id)
	return httpx.OK(c, res, err)
}

func (h *Handler) Settings(c *fiber.Ctx) error {
	res, err := h.svc.Settings(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) SaveSettings(c *fiber.Ctx) error {
	var req dto.SettingsRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.SaveSettings(c.UserContext(), middleware.BranchID(c), req)
	return httpx.OK(c, res, err)
}

// OpenByToken answers the payment still open for a booking (the page shows it again), or null
func (h *Handler) OpenByToken(c *fiber.Ctx) error {
	res, err := h.svc.OpenByToken(c.UserContext(), c.Params("slug"), c.Params("token"))
	return httpx.OK(c, res, err)
}

func (h *Handler) OpenByCustomer(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.OpenByCustomer(c.UserContext(), middleware.CurrentUser(c).ID, id)
	return httpx.OK(c, res, err)
}

// ---------- course passes ----------

func (h *Handler) StartPackage(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.StartRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.StartPackage(c.UserContext(), middleware.CurrentUser(c).ID, id, req)
	return httpx.Created(c, res, err)
}

func (h *Handler) OpenPackage(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.OpenPackage(c.UserContext(), middleware.CurrentUser(c).ID, id)
	return httpx.OK(c, res, err)
}

func (h *Handler) CheckPackage(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	pid, err := httpx.ID(c, "pid")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.CheckPackage(c.UserContext(), middleware.CurrentUser(c).ID, id, pid)
	return httpx.OK(c, res, err)
}

func (h *Handler) ForPurchase(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.ForPurchase(c.UserContext(), middleware.BranchID(c), id)
	return httpx.OK(c, res, err)
}
