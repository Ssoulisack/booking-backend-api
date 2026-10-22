package application

import (
	"backend/internal/adapters/primary/http/router"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app fiber.Router, handlers *AppHandler) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World! 👋")
	})
	// API group
	api := app.Group("/api/v1")
	h := handlers
	router.SetupAuthRoutes(api, h.auth, h.authMiddleware)
	router.SetupUserRoutes(api, h.user)
	router.SetupCompanyRoutes(api, h.authMiddleware, h.company)
	router.SetupManageRoutes(api, h.authMiddleware, h.catalog, h.schedule, h.rules, h.media, h.branch, h.desk, h.reports)
	router.SetupPublicRoutes(api, h.authMiddleware, h.invitations, h.public)
	router.SetupBookingRoutes(api, h.authMiddleware, h.booking)
}
