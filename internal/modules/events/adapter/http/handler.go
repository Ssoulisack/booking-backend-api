package http

import (
	"backend/internal/modules/events/dto"
	"backend/internal/modules/events/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the module's routes (the driving side: it only knows the input port)
type Handler struct{ svc port.Service }

// New builds the handler
func New(svc port.Service) *Handler { return &Handler{svc: svc} }

// ---------- staff ----------

func (h *Handler) List(c *fiber.Ctx) error {
	res, err := h.svc.List(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Get(c.UserContext(), middleware.BranchID(c), id)
	return httpx.OK(c, res, err)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req dto.EventRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Create(c.UserContext(), middleware.BranchID(c), req)
	return httpx.Created(c, res, err)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.EventRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Update(c.UserContext(), middleware.BranchID(c), id, req)
	return httpx.OK(c, res, err)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "EVENT_DELETED", h.svc.Delete(c.UserContext(), middleware.BranchID(c), id))
}

func (h *Handler) Copy(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.CopyRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Copy(c.UserContext(), middleware.BranchID(c), id, req)
	return httpx.Created(c, res, err)
}

func (h *Handler) Prices(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Prices(c.UserContext(), middleware.BranchID(c), id)
	return httpx.OK(c, res, err)
}

func (h *Handler) SetPrices(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.PricesRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.SetPrices(c.UserContext(), middleware.BranchID(c), id, req)
	return httpx.OK(c, res, err)
}

func (h *Handler) Pay(c *fiber.Ctx) error {
	res, err := h.svc.Pay(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) SavePay(c *fiber.Ctx) error {
	var req dto.PayInfoRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.SavePay(c.UserContext(), middleware.BranchID(c), req)
	return httpx.OK(c, res, err)
}

// ---------- public ----------

func (h *Handler) PublicList(c *fiber.Ctx) error {
	res, err := h.svc.Upcoming(c.UserContext(), c.Params("slug"))
	return httpx.OK(c, res, err)
}

func (h *Handler) PublicDetail(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Detail(c.UserContext(), c.Params("slug"), id, c.Query("date"))
	return httpx.OK(c, res, err)
}

func (h *Handler) PublicBook(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.BookRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Book(c.UserContext(), c.Params("slug"), id, middleware.CurrentUser(c), req)
	return httpx.Created(c, res, err)
}
