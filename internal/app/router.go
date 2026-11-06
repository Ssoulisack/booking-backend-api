package app

import (
	"backend/internal/platform/storage"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app fiber.Router, handlers *AppHandler) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World! 👋")
	})
	// Uploaded pictures, when they are kept on this machine. Keys are unique, so files never change.
	if handlers.filesDir != "" {
		app.Static(storage.FilesRoute, handlers.filesDir, fiber.Static{MaxAge: 365 * 24 * 60 * 60})
	}
	// API group
	api := app.Group("/api/v1")
	h := handlers
	h.identity.Routes(api, h.authMiddleware)
	h.company.Routes(api, h.authMiddleware)
	h.catalog.Routes(api, h.authMiddleware)
	h.media.Routes(api, h.authMiddleware)
	h.reports.Routes(api, h.authMiddleware)
	h.booking.Routes(api, h.authMiddleware)
	h.restaurant.Routes(api, h.authMiddleware)
	h.events.Routes(api, h.authMiddleware)
	h.payments.Routes(api, h.authMiddleware)
	h.packages.Routes(api, h.authMiddleware)
	if h.telegram != nil {
		h.telegram.Routes(api, h.authMiddleware)
	}
}
