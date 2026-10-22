package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type (
	// InvitationHandler serves invite links (no branch membership needed)
	InvitationHandler interface {
		Get(c *fiber.Ctx) error
		Accept(c *fiber.Ctx) error
	}
	invitationHandler struct {
		svc input.InvitationService
	}

	// PublicHandler serves a published branch's booking page data to anyone
	PublicHandler interface {
		Company(c *fiber.Ctx) error
		Directory(c *fiber.Ctx) error
		Branch(c *fiber.Ctx) error
		Sports(c *fiber.Ctx) error
		Resources(c *fiber.Ctx) error
		Coaches(c *fiber.Ctx) error
		ClassSessions(c *fiber.Ctx) error
	}
	publicHandler struct {
		svc input.PublicService
	}
)

func NewInvitationHandler(svc input.InvitationService) InvitationHandler {
	return &invitationHandler{svc: svc}
}

func (h *invitationHandler) Get(c *fiber.Ctx) error {
	res, err := h.svc.GetInvitation(c.UserContext(), c.Params("token"))
	return ok(c, res, err)
}

// Accept needs a login only when the invited email already has an account
func (h *invitationHandler) Accept(c *fiber.Ctx) error {
	var req dto.AcceptInvitationRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.Accept(c.UserContext(), c.Params("token"), middleware.CurrentUser(c), req)
	return ok(c, res, err)
}

func NewPublicHandler(svc input.PublicService) PublicHandler {
	return &publicHandler{svc: svc}
}

func sportIDQuery(c *fiber.Ctx) uint { return uint(max(c.QueryInt("sport_id", 0), 0)) }

// Company is the brand for the public home
func (h *publicHandler) Company(c *fiber.Ctx) error {
	res, err := h.svc.Company(c.UserContext())
	return ok(c, res, err)
}

// Directory lists live branches: ?keyword=&sport_type=&province=&page=&limit=
func (h *publicHandler) Directory(c *fiber.Ctx) error {
	var q dto.DirectoryQuery
	if err := c.QueryParser(&q); err != nil {
		return fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.svc.Directory(c.UserContext(), q)
	return paged(c, res, err)
}

func (h *publicHandler) Branch(c *fiber.Ctx) error {
	res, err := h.svc.Branch(c.UserContext(), c.Params("slug"))
	return ok(c, res, err)
}

func (h *publicHandler) Sports(c *fiber.Ctx) error {
	res, err := h.svc.Sports(c.UserContext(), c.Params("slug"))
	return ok(c, res, err)
}

func (h *publicHandler) Resources(c *fiber.Ctx) error {
	res, err := h.svc.Resources(c.UserContext(), c.Params("slug"), sportIDQuery(c))
	return ok(c, res, err)
}

func (h *publicHandler) Coaches(c *fiber.Ctx) error {
	res, err := h.svc.Coaches(c.UserContext(), c.Params("slug"), sportIDQuery(c))
	return ok(c, res, err)
}

func (h *publicHandler) ClassSessions(c *fiber.Ctx) error {
	q, err := listQuery(c)
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.ClassSessions(c.UserContext(), c.Params("slug"), q)
	return paged(c, res, err)
}
