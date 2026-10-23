package router

import (
	handler "backend/internal/adapters/primary/http/handler"
	"backend/internal/adapters/primary/http/middleware"
	"backend/pkg/jwt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SetupAuthRoutes registers all authentication related routes
func SetupAuthRoutes(router fiber.Router, authHandler handler.AuthHandler, auth *middleware.Auth) {
	a := router.Group("/auth")

	// Public - routes that don't require authentication; rate limited per client IP
	// against password guessing and email flooding
	a.Post("/register", middleware.RateLimit(1000, time.Hour), authHandler.Register)
	a.Post("/login", middleware.RateLimit(1000, 15*time.Minute), authHandler.Login)
	a.Post("/forgot-password", middleware.RateLimit(1000, 15*time.Minute), authHandler.ForgotPassword)
	a.Post("/reset-password", middleware.RateLimit(1000, 15*time.Minute), authHandler.ResetPassword)

	// Private - routes that require authentication
	a.Post("/refresh", jwt.AccessRefreshToken, authHandler.Refresh)
	a.Get("/me", auth.RequireUser, authHandler.Me)
	a.Put("/me", auth.RequireUser, authHandler.UpdateMe)
	router.Get("/me/branches", auth.RequireUser, authHandler.MyBranches)
}
