package middleware

import (
	"backend/internal/core/dto"
	"backend/pkg/response"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimit allows at most max requests per window from one client IP on the route it guards.
// Counts are kept in memory, so with several API instances each instance counts separately.
// Behind a proxy or load balancer, the server must trust its X-Forwarded-For header, or every
// client shares the proxy's IP and one limit.
func RateLimit(max int, window time.Duration) fiber.Handler {
	return RateLimitBy(max, window, func(c *fiber.Ctx) string { return c.IP() })
}

// RateLimitBy is RateLimit counting under a key you choose, instead of the IP. Use it where many people share
// one IP (a venue's Wi-Fi) and each has something of their own to count under, such as a booking's private
// token or a user id; keep a looser RateLimit beside it as the flood guard, because a key the caller picks
// (random tokens) can dodge a limit that counts only by key.
func RateLimitBy(max int, window time.Duration, key func(*fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   window,
		KeyGenerator: key,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(dto.BaseResponse{
				Success: false,
				Message: response.ErrMsgRateLimitExceeded,
			})
		},
	})
}
