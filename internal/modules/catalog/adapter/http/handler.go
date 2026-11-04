// Package http: the catalog module's driving adapter: what a branch offers (sports or areas, courts
// or tables, coaches), when (opening hours, closures, prices, classes and weekly timetables) and
// the booking rules.
package http

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/modules/catalog/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the catalog routes (it only knows the input ports)
type Handler struct {
	catalog  port.CatalogService
	schedule port.ScheduleService
	rules    port.RuleService
}

// New builds the handler
func New(c port.CatalogService, s port.ScheduleService, r port.RuleService) *Handler {
	return &Handler{catalog: c, schedule: s, rules: r}
}

// scopeParams reads /:scopeType/:scopeId; the default rule uses id 0
func scopeParams(c *fiber.Ctx) (string, uint, error) {
	id, err := c.ParamsInt("scopeId")
	if err != nil || id < 0 {
		return "", 0, response.ErrorBadRequest("INVALID_ID")
	}
	return c.Params("scopeType"), uint(id), nil
}

func (h *Handler) CreateSport(c *fiber.Ctx) error {
	return httpx.ScopedCreate(c, h.catalog.CreateSport)
}
func (h *Handler) GetSport(c *fiber.Ctx) error   { return httpx.ScopedGet(c, h.catalog.GetSport) }
func (h *Handler) ListSports(c *fiber.Ctx) error { return httpx.ScopedList(c, h.catalog.ListSports) }
func (h *Handler) UpdateSport(c *fiber.Ctx) error {
	return httpx.ScopedUpdate(c, h.catalog.UpdateSport)
}
func (h *Handler) DeleteSport(c *fiber.Ctx) error {
	return httpx.ScopedRemove(c, h.catalog.DeleteSport)
}

func (h *Handler) GetFloorPlan(c *fiber.Ctx) error { return httpx.ScopedGet(c, h.catalog.GetFloorPlan) }
func (h *Handler) SaveFloorPlan(c *fiber.Ctx) error {
	return httpx.ScopedUpdate(c, h.catalog.SaveFloorPlan)
}

func (h *Handler) CreateResource(c *fiber.Ctx) error {
	return httpx.ScopedCreate(c, h.catalog.CreateResource)
}
func (h *Handler) GetResource(c *fiber.Ctx) error { return httpx.ScopedGet(c, h.catalog.GetResource) }
func (h *Handler) ListResources(c *fiber.Ctx) error {
	return httpx.ScopedList(c, h.catalog.ListResources)
}
func (h *Handler) UpdateResource(c *fiber.Ctx) error {
	return httpx.ScopedUpdate(c, h.catalog.UpdateResource)
}
func (h *Handler) DeleteResource(c *fiber.Ctx) error {
	return httpx.ScopedRemove(c, h.catalog.DeleteResource)
}

func (h *Handler) CreateCoach(c *fiber.Ctx) error {
	return httpx.ScopedCreate(c, h.catalog.CreateCoach)
}
func (h *Handler) GetCoach(c *fiber.Ctx) error    { return httpx.ScopedGet(c, h.catalog.GetCoach) }
func (h *Handler) ListCoaches(c *fiber.Ctx) error { return httpx.ScopedList(c, h.catalog.ListCoaches) }
func (h *Handler) UpdateCoach(c *fiber.Ctx) error {
	return httpx.ScopedUpdate(c, h.catalog.UpdateCoach)
}
func (h *Handler) DeleteCoach(c *fiber.Ctx) error {
	return httpx.ScopedRemove(c, h.catalog.DeleteCoach)
}

func (h *Handler) GetOpeningHours(c *fiber.Ctx) error {
	ownerType, ownerID, err := httpx.OwnerParams(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.schedule.GetOpeningHours(c.UserContext(), middleware.BranchID(c), ownerType, ownerID)
	return httpx.OK(c, res, err)
}

func (h *Handler) SetOpeningHours(c *fiber.Ctx) error {
	ownerType, ownerID, err := httpx.OwnerParams(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.OpeningHoursRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.schedule.SetOpeningHours(c.UserContext(), middleware.BranchID(c), ownerType, ownerID, req)
	return httpx.OK(c, res, err)
}

func (h *Handler) CreateClosure(c *fiber.Ctx) error {
	return httpx.ScopedCreate(c, h.schedule.CreateClosure)
}
func (h *Handler) GetClosure(c *fiber.Ctx) error { return httpx.ScopedGet(c, h.schedule.GetClosure) }
func (h *Handler) ListClosures(c *fiber.Ctx) error {
	return httpx.ScopedList(c, h.schedule.ListClosures)
}
func (h *Handler) UpdateClosure(c *fiber.Ctx) error {
	return httpx.ScopedUpdate(c, h.schedule.UpdateClosure)
}
func (h *Handler) DeleteClosure(c *fiber.Ctx) error {
	return httpx.ScopedRemove(c, h.schedule.DeleteClosure)
}

func (h *Handler) GetPrice(c *fiber.Ctx) error {
	ownerType, ownerID, err := httpx.OwnerParams(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.schedule.GetPrice(c.UserContext(), middleware.BranchID(c), ownerType, ownerID)
	return httpx.OK(c, res, err)
}

func (h *Handler) SetPrice(c *fiber.Ctx) error {
	ownerType, ownerID, err := httpx.OwnerParams(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.PriceRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.schedule.SetPrice(c.UserContext(), middleware.BranchID(c), ownerType, ownerID, req)
	return httpx.OK(c, res, err)
}

func (h *Handler) DeletePrice(c *fiber.Ctx) error {
	ownerType, ownerID, err := httpx.OwnerParams(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "DELETED", h.schedule.DeletePrice(c.UserContext(), middleware.BranchID(c), ownerType, ownerID))
}

func (h *Handler) CreateClassSession(c *fiber.Ctx) error {
	return httpx.ScopedCreate(c, h.schedule.CreateClassSession)
}
func (h *Handler) GetClassSession(c *fiber.Ctx) error {
	return httpx.ScopedGet(c, h.schedule.GetClassSession)
}
func (h *Handler) ListClassSessions(c *fiber.Ctx) error {
	return httpx.ScopedList(c, h.schedule.ListClassSessions)
}
func (h *Handler) UpdateClassSession(c *fiber.Ctx) error {
	return httpx.ScopedUpdate(c, h.schedule.UpdateClassSession)
}
func (h *Handler) DeleteClassSession(c *fiber.Ctx) error {
	return httpx.ScopedRemove(c, h.schedule.DeleteClassSession)
}

// RepeatClassSessions creates a class on every chosen weekday between two dates
func (h *Handler) RepeatClassSessions(c *fiber.Ctx) error {
	return httpx.ScopedCreate(c, h.schedule.RepeatClassSessions)
}

// ---------- weekly class timetables ----------

func (h *Handler) ListTimetables(c *fiber.Ctx) error {
	res, err := h.schedule.ListTimetables(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

// CreateTimetable saves a weekly class and makes its classes for the next weeks
func (h *Handler) CreateTimetable(c *fiber.Ctx) error {
	return httpx.ScopedCreate(c, h.schedule.CreateTimetable)
}

func (h *Handler) UpdateTimetable(c *fiber.Ctx) error {
	return httpx.ScopedUpdate(c, h.schedule.UpdateTimetable)
}

// DeleteTimetable stops a timetable and answers how many upcoming classes were removed or kept
func (h *Handler) DeleteTimetable(c *fiber.Ctx) error {
	return httpx.ScopedGet(c, h.schedule.DeleteTimetable)
}

func (h *Handler) ListRules(c *fiber.Ctx) error {
	res, err := h.rules.ListRules(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) GetRule(c *fiber.Ctx) error {
	scopeType, scopeID, err := scopeParams(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.rules.GetRule(c.UserContext(), middleware.BranchID(c), scopeType, scopeID)
	return httpx.OK(c, res, err)
}

// CompanyRule shows a branch's team the company rule its own rules start from (read only)
func (h *Handler) CompanyRule(c *fiber.Ctx) error {
	res, err := h.rules.GetRule(c.UserContext(), 0, domain.ScopeCompany, 0)
	return httpx.OK(c, res, err)
}

func (h *Handler) SetRule(c *fiber.Ctx) error {
	scopeType, scopeID, err := scopeParams(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.RuleRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.rules.SetRule(c.UserContext(), middleware.BranchID(c), scopeType, scopeID, req)
	return httpx.OK(c, res, err)
}

func (h *Handler) DeleteRule(c *fiber.Ctx) error {
	scopeType, scopeID, err := scopeParams(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "DELETED", h.rules.DeleteRule(c.UserContext(), middleware.BranchID(c), scopeType, scopeID))
}

// Effective answers "which rule applies?" for ?sport_id= or ?resource_id=
func (h *Handler) Effective(c *fiber.Ctx) error {
	sportID := uint(max(c.QueryInt("sport_id", 0), 0))
	resourceID := uint(max(c.QueryInt("resource_id", 0), 0))
	res, err := h.rules.Effective(c.UserContext(), middleware.BranchID(c), sportID, resourceID)
	return httpx.OK(c, res, err)
}

// GetCompanyRule and SetCompanyRule are the company default booking rule (every branch starts from it)
func (h *Handler) GetCompanyRule(c *fiber.Ctx) error {
	res, err := h.rules.GetRule(c.UserContext(), 0, domain.ScopeCompany, 0)
	return httpx.OK(c, res, err)
}

func (h *Handler) SetCompanyRule(c *fiber.Ctx) error {
	var req dto.RuleRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.rules.SetRule(c.UserContext(), 0, domain.ScopeCompany, 0, req)
	return httpx.OK(c, res, err)
}
