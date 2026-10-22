package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type (
	RuleHandler interface {
		ListRules(c *fiber.Ctx) error
		GetRule(c *fiber.Ctx) error
		SetRule(c *fiber.Ctx) error
		DeleteRule(c *fiber.Ctx) error
		Effective(c *fiber.Ctx) error
	}
	ruleHandler struct {
		svc input.RuleService
	}
)

func NewRuleHandler(svc input.RuleService) RuleHandler {
	return &ruleHandler{svc: svc}
}

// scopeParams reads /:scopeType/:scopeId; the default rule uses id 0
func scopeParams(c *fiber.Ctx) (string, uint, error) {
	id, err := c.ParamsInt("scopeId")
	if err != nil || id < 0 {
		return "", 0, response.ErrorBadRequest("INVALID_ID")
	}
	return c.Params("scopeType"), uint(id), nil
}

func (h *ruleHandler) ListRules(c *fiber.Ctx) error {
	res, err := h.svc.ListRules(c.UserContext(), middleware.BranchID(c))
	return ok(c, res, err)
}

func (h *ruleHandler) GetRule(c *fiber.Ctx) error {
	scopeType, scopeID, err := scopeParams(c)
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.GetRule(c.UserContext(), middleware.BranchID(c), scopeType, scopeID)
	return ok(c, res, err)
}

func (h *ruleHandler) SetRule(c *fiber.Ctx) error {
	scopeType, scopeID, err := scopeParams(c)
	if err != nil {
		return fail(c, err)
	}
	var req dto.RuleRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.SetRule(c.UserContext(), middleware.BranchID(c), scopeType, scopeID, req)
	return ok(c, res, err)
}

func (h *ruleHandler) DeleteRule(c *fiber.Ctx) error {
	scopeType, scopeID, err := scopeParams(c)
	if err != nil {
		return fail(c, err)
	}
	return done(c, "DELETED", h.svc.DeleteRule(c.UserContext(), middleware.BranchID(c), scopeType, scopeID))
}

// Effective answers "which rule applies?" for ?sport_id= or ?resource_id=
func (h *ruleHandler) Effective(c *fiber.Ctx) error {
	sportID := uint(max(c.QueryInt("sport_id", 0), 0))
	resourceID := uint(max(c.QueryInt("resource_id", 0), 0))
	res, err := h.svc.Effective(c.UserContext(), middleware.BranchID(c), sportID, resourceID)
	return ok(c, res, err)
}
