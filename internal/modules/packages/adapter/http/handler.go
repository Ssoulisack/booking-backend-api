package http

import (
	"backend/internal/modules/packages/dto"
	"backend/internal/modules/packages/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the module's routes (it only knows the input port)
type Handler struct{ svc port.Service }

// New builds the handler
func New(svc port.Service) *Handler { return &Handler{svc: svc} }

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
	var req dto.PackageRequest
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
	var req dto.PackageRequest
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
	return httpx.Done(c, "PACKAGE_DELETED", h.svc.Delete(c.UserContext(), middleware.BranchID(c), id))
}

func (h *Handler) Public(c *fiber.Ctx) error {
	res, err := h.svc.Public(c.UserContext(), c.Params("slug"))
	return httpx.OK(c, res, err)
}

// ---------- customers ----------

func (h *Handler) Buy(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.BuyRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Buy(c.UserContext(), c.Params("slug"), middleware.CurrentUser(c), id, req)
	return httpx.Created(c, res, err)
}

func (h *Handler) Mine(c *fiber.Ctx) error {
	res, err := h.svc.Mine(c.UserContext(), middleware.CurrentUser(c).ID)
	return httpx.OK(c, res, err)
}

func (h *Handler) SlipSent(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	slip, err := httpx.SlipUpload(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.SlipSent(c.UserContext(), middleware.CurrentUser(c).ID, id, slip)
	return httpx.OK(c, res, err)
}

func (h *Handler) CancelMine(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.CancelMine(c.UserContext(), middleware.CurrentUser(c).ID, id)
	return httpx.OK(c, res, err)
}

// ---------- staff: packages customers own ----------

func (h *Handler) Sold(c *fiber.Ctx) error {
	res, err := h.svc.Sold(c.UserContext(), middleware.BranchID(c), uint(c.QueryInt("customer_id", 0)), c.Query("status"), c.Query("q"))
	return httpx.OK(c, res, err)
}

func (h *Handler) Sell(c *fiber.Ctx) error {
	var req dto.SellRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Sell(c.UserContext(), middleware.BranchID(c), middleware.CurrentUser(c), req)
	return httpx.Created(c, res, err)
}

func (h *Handler) MarkPaid(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.MarkPaidRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.MarkPaid(c.UserContext(), middleware.BranchID(c), id, middleware.CurrentUser(c), req)
	return httpx.OK(c, res, err)
}

func (h *Handler) Cancel(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Cancel(c.UserContext(), middleware.BranchID(c), id, middleware.CurrentUser(c))
	return httpx.OK(c, res, err)
}

// ---------- the log and adjustments ----------

func (h *Handler) Visits(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Visits(c.UserContext(), middleware.BranchID(c), id)
	return httpx.OK(c, res, err)
}

func (h *Handler) MyVisits(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.MyVisits(c.UserContext(), middleware.CurrentUser(c).ID, id)
	return httpx.OK(c, res, err)
}

func (h *Handler) Adjust(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.AdjustRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.svc.Adjust(c.UserContext(), middleware.BranchID(c), id, middleware.CurrentUser(c), req)
	return httpx.OK(c, res, err)
}
