package limiter

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// StrictRateLimit creates a strict rate limiting middleware (10 requests per minute)
func StrictRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        10,              // Maximum 10 requests
		Expiration: 1 * time.Minute, // Per minute
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"status": false,
				"error":  "Rate limit exceeded. Maximum 10 requests per minute",
			})
		},
	})
}

// LooseRateLimit creates a loose rate limiting middleware (100 requests per hour)
func LooseRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        100,           // Maximum 100 requests
		Expiration: 1 * time.Hour, // Per hour
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"status": false,
				"error":  "Rate limit exceeded. Maximum 100 requests per hour.",
			})
		},
	})
}

// AuthRateLimit creates a rate limiting middleware for authentication endpoints
func AuthRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        3,                // Maximum 3 login attempts
		Expiration: 15 * time.Minute, // Per 15 minutes
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"status": false,
				"error":  "Too many login attempts. Please try again in 15 minutes.",
			})
		},
	})
}

// CustomRateLimit creates a custom rate limiting middleware with configurable parameters
func CustomRateLimit(max int, expiration time.Duration, keyGenerator func(c *fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   expiration,
		KeyGenerator: keyGenerator,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"status": false,
				"error":  "Rate limit exceeded.",
			})
		},
	})
}
