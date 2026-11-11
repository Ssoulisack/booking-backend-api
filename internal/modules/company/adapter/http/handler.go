// Package http: the company module's driving adapter: the company's settings, branches, team, users
// and customers (owner and admins), one branch's own settings and publishing, and the public site
// (brand, list of places, a place's sports, courts, coaches and classes).
package http

import (
	"backend/internal/core/dto"
	"backend/internal/modules/company/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"
	"backend/pkg/response"
	"context"

	"github.com/gofiber/fiber/v2"
)

// Customers lists the accounts who booked (the booking module's desk answers it)
type Customers interface {
	Customers(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.CustomerDTO], error)
}

// Handler answers the company routes (it only knows the input ports)
type Handler struct {
	company   port.CompanyService
	branch    port.BranchService
	public    port.PublicService
	customers Customers
}

// New builds the handler
func New(company port.CompanyService, branch port.BranchService, public port.PublicService, customers Customers) *Handler {
	return &Handler{company: company, branch: branch, public: public, customers: customers}
}

func (h *Handler) GetCompany(c *fiber.Ctx) error {
	res, err := h.company.GetCompany(c.UserContext())
	return httpx.OK(c, res, err)
}

func (h *Handler) UpdateCompany(c *fiber.Ctx) error {
	var req dto.CompanyRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.company.UpdateCompany(c.UserContext(), req)
	return httpx.OK(c, res, err)
}

// ---------- branches ----------

func (h *Handler) ListBranches(c *fiber.Ctx) error {
	res, err := h.company.ListBranches(c.UserContext())
	return httpx.OK(c, res, err)
}

func (h *Handler) CreateBranch(c *fiber.Ctx) error {
	var req dto.CreateBranchRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.company.CreateBranch(c.UserContext(), req)
	return httpx.Created(c, res, err)
}

func (h *Handler) setClosed(c *fiber.Ctx, closed bool) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.company.SetClosed(c.UserContext(), id, closed)
	return httpx.OK(c, res, err)
}

func (h *Handler) CloseBranch(c *fiber.Ctx) error  { return h.setClosed(c, true) }
func (h *Handler) ReopenBranch(c *fiber.Ctx) error { return h.setClosed(c, false) }

// ---------- team ----------

func (h *Handler) ListTeam(c *fiber.Ctx) error {
	res, err := h.company.ListTeam(c.UserContext())
	return httpx.OK(c, res, err)
}

func (h *Handler) UpdateTeamMember(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "userId")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.TeamMemberRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.company.UpdateTeamMember(c.UserContext(), middleware.CurrentUser(c), id, req)
	return httpx.OK(c, res, err)
}

// FindTeamCandidate: ?q= a phone or member number
func (h *Handler) FindTeamCandidate(c *fiber.Ctx) error {
	res, err := h.company.FindTeamCandidate(c.UserContext(), c.Query("q"))
	return httpx.OK(c, res, err)
}

// AddTeamMember puts a registered account on the team (role and branches), without an invitation
func (h *Handler) AddTeamMember(c *fiber.Ctx) error {
	var req dto.AddTeamMemberRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.company.AddTeamMember(c.UserContext(), middleware.CurrentUser(c), req)
	return httpx.Created(c, res, err)
}

// BranchTeam: ?role=manager&q=2055551234
func (h *Handler) BranchTeam(c *fiber.Ctx) error {
	branchID, err := httpx.ID(c, "branchId")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.company.BranchTeam(c.UserContext(), branchID, c.Query("role"), c.Query("q"))
	return httpx.OK(c, res, err)
}

// SetBranchAccess: {"active": false} switches the person's access to this branch off
func (h *Handler) SetBranchAccess(c *fiber.Ctx) error {
	branchID, err := httpx.ID(c, "branchId")
	if err != nil {
		return httpx.Fail(c, err)
	}
	userID, err := httpx.ID(c, "userId")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "UPDATED", h.company.SetBranchAccess(c.UserContext(), middleware.CurrentUser(c), branchID, userID, req.Active))
}

// RemoveFromBranch takes the person off this branch
func (h *Handler) RemoveFromBranch(c *fiber.Ctx) error {
	branchID, err := httpx.ID(c, "branchId")
	if err != nil {
		return httpx.Fail(c, err)
	}
	userID, err := httpx.ID(c, "userId")
	if err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "REMOVED", h.company.RemoveFromBranch(c.UserContext(), middleware.CurrentUser(c), branchID, userID))
}

func (h *Handler) RemoveTeamMember(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "userId")
	if err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "REMOVED", h.company.RemoveTeamMember(c.UserContext(), middleware.CurrentUser(c), id))
}

func (h *Handler) TransferOwnership(c *fiber.Ctx) error {
	var req dto.TransferOwnershipRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "OWNERSHIP_TRANSFERRED", h.company.TransferOwnership(c.UserContext(), middleware.CurrentUser(c), req.UserID))
}

// ---------- users ----------

func (h *Handler) ListUsers(c *fiber.Ctx) error {
	q, err := httpx.ListQuery(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.company.ListUsers(c.UserContext(), q)
	return httpx.Paged(c, res, err)
}

func (h *Handler) SetUserStatus(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.UserStatusRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "UPDATED", h.company.SetUserStatus(c.UserContext(), middleware.CurrentUser(c), id, req.Status))
}

// SetUserPassword sets a new password for someone who cannot reset it by email (no real address)
func (h *Handler) SetUserPassword(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	var req dto.PasswordRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "PASSWORD_UPDATED", h.company.SetUserPassword(c.UserContext(), middleware.CurrentUser(c), id, req.Password))
}

// ---------- customers and report ----------

// Customers lists accounts who booked at any branch
func (h *Handler) Customers(c *fiber.Ctx) error {
	q, err := httpx.ListQuery(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.customers.Customers(c.UserContext(), 0, q)
	return httpx.Paged(c, res, err)
}

func (h *Handler) GetSettings(c *fiber.Ctx) error {
	res, err := h.branch.GetSettings(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) UpdateSettings(c *fiber.Ctx) error {
	var req dto.SettingsRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.branch.UpdateSettings(c.UserContext(), middleware.BranchID(c), req)
	return httpx.OK(c, res, err)
}

func (h *Handler) Publish(c *fiber.Ctx) error {
	res, err := h.branch.Publish(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) Unpublish(c *fiber.Ctx) error {
	res, err := h.branch.Unpublish(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) GetTableSettings(c *fiber.Ctx) error {
	res, err := h.branch.GetTableSettings(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) UpdateTableSettings(c *fiber.Ctx) error {
	var req dto.TableSettingsDTO
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.branch.UpdateTableSettings(c.UserContext(), middleware.BranchID(c), req)
	return httpx.OK(c, res, err)
}

func sportIDQuery(c *fiber.Ctx) uint { return uint(max(c.QueryInt("sport_id", 0), 0)) }

func (h *Handler) PublicCompany(c *fiber.Ctx) error {
	res, err := h.public.Company(c.UserContext())
	return httpx.OK(c, res, err)
}

// Directory lists live branches: ?keyword=&sport_type=&province=&page=&limit=
func (h *Handler) Directory(c *fiber.Ctx) error {
	var q dto.DirectoryQuery
	if err := c.QueryParser(&q); err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.public.Directory(c.UserContext(), q)
	return httpx.Paged(c, res, err)
}

func (h *Handler) PublicBranch(c *fiber.Ctx) error {
	res, err := h.public.Branch(c.UserContext(), c.Params("slug"))
	return httpx.OK(c, res, err)
}

func (h *Handler) Sports(c *fiber.Ctx) error {
	res, err := h.public.Sports(c.UserContext(), c.Params("slug"))
	return httpx.OK(c, res, err)
}

func (h *Handler) Resources(c *fiber.Ctx) error {
	res, err := h.public.Resources(c.UserContext(), c.Params("slug"), sportIDQuery(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) Coaches(c *fiber.Ctx) error {
	res, err := h.public.Coaches(c.UserContext(), c.Params("slug"), sportIDQuery(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) ClassSessions(c *fiber.Ctx) error {
	q, err := httpx.ListQuery(c)
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.public.ClassSessions(c.UserContext(), c.Params("slug"), q)
	return httpx.Paged(c, res, err)
}
