package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Routes registers what the company owner and admins run under /company (the owner-only actions
// are checked in the service), one branch's own settings under /branches/:branchId, and the public
// site anyone can call.
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	staff := auth.Branch(domain.MemberStaff)
	manager := auth.Branch(domain.MemberManager)
	admin := auth.Branch(domain.CompanyAdmin)

	c := api.Group("/company", auth.RequireUser, auth.RequireCompany)
	c.Get("/", h.GetCompany)
	c.Put("/", h.UpdateCompany)

	c.Get("/branches", h.ListBranches)
	c.Post("/branches", h.CreateBranch)
	c.Post("/branches/:id/close", h.CloseBranch)
	c.Post("/branches/:id/reopen", h.ReopenBranch)

	c.Get("/team", h.ListTeam)
	c.Get("/team/lookup", h.FindTeamCandidate)
	c.Post("/team", h.AddTeamMember)
	c.Put("/team/:userId", h.UpdateTeamMember)
	c.Delete("/team/:userId", h.RemoveTeamMember)
	c.Post("/transfer-ownership", h.TransferOwnership)

	c.Get("/users", h.ListUsers)
	c.Patch("/users/:id/status", h.SetUserStatus)
	c.Post("/users/:id/password", h.SetUserPassword)

	c.Get("/customers", h.Customers)

	// the branch's own settings (company values in force are shown with them) and publishing
	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Get("/settings", staff, h.GetSettings)
	// who works here: managers see the list; the owner and admins switch access off or remove someone
	b.Get("/team", manager, h.BranchTeam)
	b.Patch("/team/:userId", admin, h.SetBranchAccess)
	b.Delete("/team/:userId", admin, h.RemoveFromBranch)
	b.Put("/settings", admin, h.UpdateSettings)
	b.Post("/publish", admin, h.Publish)
	b.Post("/unpublish", admin, h.Unpublish)
	// restaurant branches: how tables are booked
	b.Get("/table-settings", staff, h.GetTableSettings)
	b.Put("/table-settings", manager, h.UpdateTableSettings)

	// the public site: the brand and live branches
	api.Get("/public/company", h.PublicCompany)
	api.Get("/public/branches", h.Directory)
	pub := api.Group("/public/branches/:slug")
	pub.Get("/", h.PublicBranch)
	pub.Get("/sports", h.Sports)
	pub.Get("/resources", h.Resources)
	pub.Get("/coaches", h.Coaches)
	pub.Get("/class-sessions", h.ClassSessions)
}
