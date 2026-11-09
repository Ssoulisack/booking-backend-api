package http

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ResolveVersion picks the number Health shows: the one in config.yaml (app.version) when it is set, else the one baked into
// the image, else "dev".
func ResolveVersion(configured, baked string) string {
	for _, v := range []string{configured, baked} {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return "dev"
}

// Health answers GET /api/v1/health for the container health check, the uptime monitor and a quick look by hand.
//
//   - 200 {"success":true,"data":{"status":"ok","version":"1.0.5","database":"ok","cache":"ok"|"off"|"down"}}
//   - 503 {"success":false,"message":"DATABASE_DOWN","data":{...}} when the database does not answer
//
// The database is required. The Redis cache is optional (the app reads the database when Redis is down), so a Redis that
// is down shows as "down" but does not turn the answer into 503. No login, no secrets, nothing about the data.
//
// dbPing and cachePing are plain functions so the handler can be tested without a database. cachePing reports whether
// the cache is switched on; nil means "no cache".
func Health(version string, dbPing func(context.Context) error, cachePing func(context.Context) (bool, error)) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		data := fiber.Map{"status": "ok", "version": version, "database": "ok", "cache": "off"}
		healthy := true

		if dbPing == nil || dbPing(ctx) != nil {
			data["status"], data["database"] = "down", "down"
			healthy = false
		}
		if cachePing != nil {
			if enabled, err := cachePing(ctx); enabled {
				if err != nil {
					data["cache"] = "down"
				} else {
					data["cache"] = "ok"
				}
			}
		}

		c.Set("Cache-Control", "no-store")
		if !healthy {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"success": false, "message": "DATABASE_DOWN", "data": data})
		}
		return c.JSON(fiber.Map{"success": true, "data": data})
	}
}
