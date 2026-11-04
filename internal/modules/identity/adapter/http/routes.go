package http

import (
	"backend/internal/platform/middleware"
	"backend/pkg/jwt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Routes registers sign up, login, the person's own account and the invite links
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	a := api.Group("/auth")

	// Public - routes that don't require authentication; rate limited per client IP
	// against password guessing and email flooding
	a.Post("/register", middleware.RateLimit(1000, time.Hour), h.Register)
	a.Post("/login", middleware.RateLimit(1000, 15*time.Minute), h.Login)
	a.Get("/social/config", h.SocialConfig)
	a.Post("/social/:provider", middleware.RateLimit(30, 15*time.Minute), h.SocialLogin)
	a.Post("/forgot-password", middleware.RateLimit(1000, 15*time.Minute), h.ForgotPassword)
	a.Post("/reset-password", middleware.RateLimit(1000, 15*time.Minute), h.ResetPassword)

	// Private - routes that require authentication
	a.Post("/refresh", jwt.AccessRefreshToken, h.Refresh)
	a.Get("/me", auth.RequireUser, h.Me)
	a.Put("/me", auth.RequireUser, h.UpdateMe)
	api.Get("/me/branches", auth.RequireUser, h.MyBranches)

	// invite links: anyone with the link
	api.Get("/invitations/:token", h.Invitation)
	api.Post("/invitations/:token/accept", middleware.RateLimit(1000, 15*time.Minute), auth.OptionalUser, h.AcceptInvitation)

	// the company owner and admins manage who is invited
	team := api.Group("/company/invitations", auth.RequireUser, auth.RequireCompany)
	team.Get("/", h.ListInvitations)
	team.Post("/", h.Invite)
	team.Delete("/:id", h.CancelInvitation)
	team.Post("/:id/resend", h.ResendInvitation)
}
