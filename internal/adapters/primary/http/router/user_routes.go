package router

import (
	handler "backend/internal/adapters/primary/http/handler"

	"github.com/gofiber/fiber/v2"
)

// RegisterAuthRoutes registers all authentication related routes
func SetupUserRoutes(router fiber.Router, userHandler handler.UserHandler) {

}
