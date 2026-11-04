package http

import (
	"backend/internal/core/domain"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Routes registers a branch's media library (managers) and the company's (owner and admins)
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	manager := auth.Branch(domain.MemberManager)

	b := api.Group("/branches/:branchId", auth.RequireUser)
	b.Get("/uploads", manager, h.BranchList)
	b.Post("/uploads", manager, h.BranchUpload)
	b.Delete("/uploads", manager, h.BranchDelete)

	c := api.Group("/company/uploads", auth.RequireUser, auth.RequireCompany)
	c.Get("/", h.CompanyList)
	c.Post("/", h.CompanyUpload)
	c.Delete("/", h.CompanyDelete)
}
