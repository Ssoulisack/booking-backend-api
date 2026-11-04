// Package http: the restaurant module's driving adapter: the public floor plans and free tables, and
// the team's table desk (arrived, move, walk-in).
package http

import (
	"backend/internal/core/dto"
	"backend/internal/modules/restaurant/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the restaurant routes (it only knows the input ports)
type Handler struct {
	tables port.TableService
	desk   port.DeskTables
}

// New builds the handler
func New(tables port.TableService, desk port.DeskTables) *Handler {
	return &Handler{tables: tables, desk: desk}
}

func (h *Handler) FloorPlans(c *fiber.Ctx) error {
	res, err := h.tables.FloorPlans(c.UserContext(), c.Params("slug"))
	return httpx.OK(c, res, err)
}

func (h *Handler) TableAvailability(c *fiber.Ctx) error {
	var q dto.TableAvailabilityQuery
	if err := c.QueryParser(&q); err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.tables.TableAvailability(c.UserContext(), c.Params("slug"), q)
	return httpx.OK(c, res, err)
}

// MarkArrived: a restaurant group came in
func (h *Handler) MarkArrived(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.desk.MarkArrived(c.UserContext(), middleware.BranchID(c), id)
	return httpx.OK(c, res, err)
}

// MoveTables puts a restaurant group at other tables
func (h *Handler) MoveTables(c *fiber.Ctx) error { return httpx.ScopedUpdate(c, h.desk.MoveTables) }

// WalkIn seats a group that came without booking
func (h *Handler) WalkIn(c *fiber.Ctx) error {
	var req dto.WalkInRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.desk.WalkIn(c.UserContext(), middleware.BranchID(c), middleware.CurrentUser(c), req)
	return httpx.Created(c, res, err)
}
