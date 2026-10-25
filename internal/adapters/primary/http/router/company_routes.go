package router

import (
	handler "backend/internal/adapters/primary/http/handler"
	"backend/internal/adapters/primary/http/middleware"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SetupCompanyRoutes registers what the company owner and admins run under /company.
// The owner-only actions (admins, ownership) are checked in the service.
func SetupCompanyRoutes(router fiber.Router, auth *middleware.Auth, company handler.CompanyHandler) {
	c := router.Group("/company", auth.RequireUser, auth.RequireCompany)

	c.Get("/", company.GetCompany)
	c.Put("/", company.UpdateCompany)
	c.Get("/rules", company.GetRule)
	c.Put("/rules", company.SetRule)
	c.Get("/uploads", company.ListUploads)
	c.Post("/uploads", company.Upload)
	c.Delete("/uploads", company.DeleteUpload)

	c.Get("/branches", company.ListBranches)
	c.Post("/branches", company.CreateBranch)
	c.Post("/branches/:id/close", company.CloseBranch)
	c.Post("/branches/:id/reopen", company.ReopenBranch)

	c.Get("/team", company.ListTeam)
	c.Put("/team/:userId", company.UpdateTeamMember)
	c.Delete("/team/:userId", company.RemoveTeamMember)
	c.Post("/transfer-ownership", company.TransferOwnership)

	c.Get("/invitations", company.ListInvitations)
	c.Post("/invitations", company.Invite)
	c.Delete("/invitations/:id", company.CancelInvitation)
	c.Post("/invitations/:id/resend", company.ResendInvitation)

	c.Get("/users", company.ListUsers)
	c.Patch("/users/:id/status", company.SetUserStatus)
	c.Post("/users/:id/password", company.SetUserPassword)

	c.Get("/customers", company.Customers)
	c.Get("/report", company.Report)
}

// SetupPublicRoutes registers routes anyone can call: invite links, the brand and live branches
func SetupPublicRoutes(
	router fiber.Router,
	auth *middleware.Auth,
	invitations handler.InvitationHandler,
	public handler.PublicHandler,
) {
	router.Get("/invitations/:token", invitations.Get)
	router.Post("/invitations/:token/accept", middleware.RateLimit(1000, 15*time.Minute), auth.OptionalUser, invitations.Accept)

	router.Get("/public/company", public.Company)
	router.Get("/public/branches", public.Directory)

	pub := router.Group("/public/branches/:slug")
	pub.Get("/", public.Branch)
	pub.Get("/sports", public.Sports)
	pub.Get("/resources", public.Resources)
	pub.Get("/coaches", public.Coaches)
	pub.Get("/class-sessions", public.ClassSessions)
}
