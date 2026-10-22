package handler

import (
	"backend/internal/core/port/input"

	"github.com/gofiber/fiber/v2"
)

type (
	CatalogHandler interface {
		CreateSport(c *fiber.Ctx) error
		GetSport(c *fiber.Ctx) error
		ListSports(c *fiber.Ctx) error
		UpdateSport(c *fiber.Ctx) error
		DeleteSport(c *fiber.Ctx) error

		CreateResource(c *fiber.Ctx) error
		GetResource(c *fiber.Ctx) error
		ListResources(c *fiber.Ctx) error
		UpdateResource(c *fiber.Ctx) error
		DeleteResource(c *fiber.Ctx) error

		CreateCoach(c *fiber.Ctx) error
		GetCoach(c *fiber.Ctx) error
		ListCoaches(c *fiber.Ctx) error
		UpdateCoach(c *fiber.Ctx) error
		DeleteCoach(c *fiber.Ctx) error
	}
	catalogHandler struct {
		svc input.CatalogService
	}
)

func NewCatalogHandler(svc input.CatalogService) CatalogHandler {
	return &catalogHandler{svc: svc}
}

func (h *catalogHandler) CreateSport(c *fiber.Ctx) error { return scopedCreate(c, h.svc.CreateSport) }
func (h *catalogHandler) GetSport(c *fiber.Ctx) error    { return scopedGet(c, h.svc.GetSport) }
func (h *catalogHandler) ListSports(c *fiber.Ctx) error  { return scopedList(c, h.svc.ListSports) }
func (h *catalogHandler) UpdateSport(c *fiber.Ctx) error { return scopedUpdate(c, h.svc.UpdateSport) }
func (h *catalogHandler) DeleteSport(c *fiber.Ctx) error { return scopedRemove(c, h.svc.DeleteSport) }

func (h *catalogHandler) CreateResource(c *fiber.Ctx) error {
	return scopedCreate(c, h.svc.CreateResource)
}
func (h *catalogHandler) GetResource(c *fiber.Ctx) error   { return scopedGet(c, h.svc.GetResource) }
func (h *catalogHandler) ListResources(c *fiber.Ctx) error { return scopedList(c, h.svc.ListResources) }
func (h *catalogHandler) UpdateResource(c *fiber.Ctx) error {
	return scopedUpdate(c, h.svc.UpdateResource)
}
func (h *catalogHandler) DeleteResource(c *fiber.Ctx) error {
	return scopedRemove(c, h.svc.DeleteResource)
}

func (h *catalogHandler) CreateCoach(c *fiber.Ctx) error { return scopedCreate(c, h.svc.CreateCoach) }
func (h *catalogHandler) GetCoach(c *fiber.Ctx) error    { return scopedGet(c, h.svc.GetCoach) }
func (h *catalogHandler) ListCoaches(c *fiber.Ctx) error { return scopedList(c, h.svc.ListCoaches) }
func (h *catalogHandler) UpdateCoach(c *fiber.Ctx) error { return scopedUpdate(c, h.svc.UpdateCoach) }
func (h *catalogHandler) DeleteCoach(c *fiber.Ctx) error { return scopedRemove(c, h.svc.DeleteCoach) }
