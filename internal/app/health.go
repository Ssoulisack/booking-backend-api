package app

import (
	"context"

	apphttp "backend/internal/http"

	"github.com/gofiber/fiber/v2"
)

// Version is the number baked into the image: the Dockerfile sets it from the `version` file at build time
// (-ldflags "-X backend/internal/app.Version=1.0.5"). It is only used when app.version in config.yaml is empty;
// with neither, the health check shows "dev".
var Version = ""

// SetupHealth adds GET /api/v1/health (see apphttp.Health).
func SetupHealth(app fiber.Router, r *AppRepositories) {
	dbPing := func(ctx context.Context) error {
		sqlDB, err := r.postgres.GetDB().DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}
	app.Get("/api/v1/health", apphttp.Health(apphttp.ResolveVersion(r.app.Version, Version), dbPing, r.cache.Ping))
}
