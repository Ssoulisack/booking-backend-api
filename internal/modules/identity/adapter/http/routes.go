package http

import (
	"backend/internal/platform/middleware"
	"backend/pkg/jwt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	codeWindow      = 15 * time.Minute
	codesPerWindow  = 5  // emails with a code, per person
	checksPerWindow = 20 // code checks, per person (each code also locks after 5 wrong tries)
)

// Routes registers sign up, login, the person's own account and the invite links
func (h *Handler) Routes(api fiber.Router, auth *middleware.Auth) {
	a := api.Group("/auth")

	// Public - routes that don't require authentication; rate limited per client IP
	// against password guessing and email flooding
	a.Post("/register", middleware.RateLimit(60, time.Hour), h.Register)
	a.Post("/login", middleware.RateLimit(120, time.Hour), h.Login)
	a.Get("/social/config", h.SocialConfig)
	a.Post("/social/:provider", middleware.RateLimit(30, 15*time.Minute), h.SocialLogin)
	a.Post("/forgot-password", middleware.RateLimit(5, 15*time.Minute), h.ForgotPassword)
	a.Post("/reset-password", middleware.RateLimit(10, 15*time.Minute), h.ResetPassword)

	// Private - routes that require authentication
	a.Post("/refresh", jwt.AccessRefreshToken, h.Refresh)
	a.Get("/me", auth.RequireUser, h.Me)
	a.Put("/me", auth.RequireUser, h.UpdateMe)
	a.Put("/me/password", middleware.RateLimit(10, 15*time.Minute), auth.RequireUser, h.ChangePassword)
	// email codes are counted per person in a 15-minute window (a venue's Wi-Fi shares one IP); the looser per-IP
	// limit stays in front as the flood guard
	perUser := func(max int) fiber.Handler {
		return middleware.RateLimitBy(max, codeWindow, func(c *fiber.Ctx) string {
			return "user:" + strconv.FormatUint(uint64(middleware.CurrentUser(c).ID), 10) + c.Path()
		})
	}
	a.Post("/me/email/code", middleware.RateLimit(30, codeWindow), auth.RequireUser, perUser(codesPerWindow), h.SendEmailCode)
	a.Post("/me/email/verify", middleware.RateLimit(60, codeWindow), auth.RequireUser, perUser(checksPerWindow), h.VerifyEmail)
	api.Get("/me/branches", auth.RequireUser, h.MyBranches)

	// invite links: anyone with the link
	api.Get("/invitations/:token", h.Invitation)
	api.Post("/invitations/:token/accept", middleware.RateLimit(10, 15*time.Minute), auth.OptionalUser, h.AcceptInvitation)

	// the company owner and admins manage who is invited
	team := api.Group("/company/invitations", auth.RequireUser, auth.RequireCompany)
	team.Get("/", h.ListInvitations)
	team.Post("/", h.Invite)
	team.Delete("/:id", h.CancelInvitation)
	team.Post("/:id/resend", h.ResendInvitation)
}
