package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type (
	// CompanyHandler serves the owner and admins: company settings and rules, branches, team,
	// users, customers and the company report
	CompanyHandler interface {
		GetCompany(c *fiber.Ctx) error
		UpdateCompany(c *fiber.Ctx) error
		GetRule(c *fiber.Ctx) error
		SetRule(c *fiber.Ctx) error
		Upload(c *fiber.Ctx) error
		DeleteUpload(c *fiber.Ctx) error

		ListBranches(c *fiber.Ctx) error
		CreateBranch(c *fiber.Ctx) error
		CloseBranch(c *fiber.Ctx) error
		ReopenBranch(c *fiber.Ctx) error

		ListTeam(c *fiber.Ctx) error
		UpdateTeamMember(c *fiber.Ctx) error
		RemoveTeamMember(c *fiber.Ctx) error
		TransferOwnership(c *fiber.Ctx) error

		ListInvitations(c *fiber.Ctx) error
		Invite(c *fiber.Ctx) error
		CancelInvitation(c *fiber.Ctx) error
		ResendInvitation(c *fiber.Ctx) error

		ListUsers(c *fiber.Ctx) error
		SetUserStatus(c *fiber.Ctx) error
		SetUserPassword(c *fiber.Ctx) error

		Customers(c *fiber.Ctx) error
		Report(c *fiber.Ctx) error
	}
	companyHandler struct {
		svc     input.CompanyService
		rules   input.RuleService
		media   input.MediaService
		desk    input.DeskService
		reports input.ReportService
	}
)

func NewCompanyHandler(
	svc input.CompanyService,
	rules input.RuleService,
	media input.MediaService,
	desk input.DeskService,
	reports input.ReportService,
) CompanyHandler {
	return &companyHandler{svc: svc, rules: rules, media: media, desk: desk, reports: reports}
}

// ---------- company ----------

func (h *companyHandler) GetCompany(c *fiber.Ctx) error {
	res, err := h.svc.GetCompany(c.UserContext())
	return ok(c, res, err)
}

func (h *companyHandler) UpdateCompany(c *fiber.Ctx) error {
	var req dto.CompanyRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.UpdateCompany(c.UserContext(), req)
	return ok(c, res, err)
}

// GetRule and SetRule are the company default booking rule (every branch starts from it)
func (h *companyHandler) GetRule(c *fiber.Ctx) error {
	res, err := h.rules.GetRule(c.UserContext(), 0, domain.ScopeCompany, 0)
	return ok(c, res, err)
}

func (h *companyHandler) SetRule(c *fiber.Ctx) error {
	var req dto.RuleRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.rules.SetRule(c.UserContext(), 0, domain.ScopeCompany, 0, req)
	return ok(c, res, err)
}

// Upload stores a company file (logo, cover): ?folder=branding
func (h *companyHandler) Upload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return fail(c, response.ErrorBadRequest("FILE_IS_REQUIRED"))
	}
	f, err := file.Open()
	if err != nil {
		return fail(c, response.ErrorBadRequest("INVALID_FILE"))
	}
	defer f.Close()
	res, err := h.media.Upload(c.UserContext(), 0, c.Query("folder"), f)
	return created(c, res, err)
}

func (h *companyHandler) DeleteUpload(c *fiber.Ctx) error {
	return done(c, "DELETED", h.media.Delete(c.UserContext(), 0, c.Query("key")))
}

// ---------- branches ----------

func (h *companyHandler) ListBranches(c *fiber.Ctx) error {
	res, err := h.svc.ListBranches(c.UserContext())
	return ok(c, res, err)
}

func (h *companyHandler) CreateBranch(c *fiber.Ctx) error {
	var req dto.CreateBranchRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.CreateBranch(c.UserContext(), req)
	return created(c, res, err)
}

func (h *companyHandler) setClosed(c *fiber.Ctx, closed bool) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.SetClosed(c.UserContext(), id, closed)
	return ok(c, res, err)
}

func (h *companyHandler) CloseBranch(c *fiber.Ctx) error  { return h.setClosed(c, true) }
func (h *companyHandler) ReopenBranch(c *fiber.Ctx) error { return h.setClosed(c, false) }

// ---------- team ----------

func (h *companyHandler) ListTeam(c *fiber.Ctx) error {
	res, err := h.svc.ListTeam(c.UserContext())
	return ok(c, res, err)
}

func (h *companyHandler) UpdateTeamMember(c *fiber.Ctx) error {
	id, err := idParam(c, "userId")
	if err != nil {
		return fail(c, err)
	}
	var req dto.TeamMemberRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.UpdateTeamMember(c.UserContext(), middleware.CurrentUser(c), id, req)
	return ok(c, res, err)
}

func (h *companyHandler) RemoveTeamMember(c *fiber.Ctx) error {
	id, err := idParam(c, "userId")
	if err != nil {
		return fail(c, err)
	}
	return done(c, "REMOVED", h.svc.RemoveTeamMember(c.UserContext(), middleware.CurrentUser(c), id))
}

func (h *companyHandler) TransferOwnership(c *fiber.Ctx) error {
	var req dto.TransferOwnershipRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	return done(c, "OWNERSHIP_TRANSFERRED", h.svc.TransferOwnership(c.UserContext(), middleware.CurrentUser(c), req.UserID))
}

// ---------- invitations ----------

func (h *companyHandler) ListInvitations(c *fiber.Ctx) error {
	res, err := h.svc.ListInvitations(c.UserContext())
	return ok(c, res, err)
}

func (h *companyHandler) Invite(c *fiber.Ctx) error {
	var req dto.InviteRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.Invite(c.UserContext(), middleware.CurrentUser(c), req)
	return created(c, res, err)
}

func (h *companyHandler) CancelInvitation(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	return done(c, "DELETED", h.svc.CancelInvitation(c.UserContext(), id))
}

func (h *companyHandler) ResendInvitation(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.ResendInvitation(c.UserContext(), middleware.CurrentUser(c), id)
	return ok(c, res, err)
}

// ---------- users ----------

func (h *companyHandler) ListUsers(c *fiber.Ctx) error {
	q, err := listQuery(c)
	if err != nil {
		return fail(c, err)
	}
	res, err := h.svc.ListUsers(c.UserContext(), q)
	return paged(c, res, err)
}

func (h *companyHandler) SetUserStatus(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req dto.UserStatusRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	return done(c, "UPDATED", h.svc.SetUserStatus(c.UserContext(), middleware.CurrentUser(c), id, req.Status))
}

// SetUserPassword sets a new password for someone who cannot reset it by email (no real address)
func (h *companyHandler) SetUserPassword(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req dto.PasswordRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	return done(c, "PASSWORD_UPDATED", h.svc.SetUserPassword(c.UserContext(), middleware.CurrentUser(c), id, req.Password))
}

// ---------- customers and report ----------

// Customers lists accounts who booked at any branch
func (h *companyHandler) Customers(c *fiber.Ctx) error {
	q, err := listQuery(c)
	if err != nil {
		return fail(c, err)
	}
	res, err := h.desk.Customers(c.UserContext(), 0, q)
	return paged(c, res, err)
}

// Report adds up every branch: ?from=&to= (YYYY-MM-DD)
func (h *companyHandler) Report(c *fiber.Ctx) error {
	var q dto.ReportQuery
	if err := c.QueryParser(&q); err != nil {
		return fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.reports.Company(c.UserContext(), q)
	return ok(c, res, err)
}
