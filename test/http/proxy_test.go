// Package httpserver checks the HTTP server settings that matter in production.
package httpserver

import (
	server "backend/internal/adapters/primary/http"
	"backend/internal/adapters/primary/http/middleware"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// newApp builds the real server with a route limited to 1 request per minute per client IP
func newApp(trusted []string) *fiber.App {
	app := server.NewServer(server.ServerConfig{ProxyHeader: fiber.HeaderXForwardedFor, TrustedProxies: trusted}).App()
	app.Get("/limited", middleware.RateLimit(1, time.Minute), func(c *fiber.Ctx) error { return c.SendString(c.IP()) })
	app.Get("/peer", func(c *fiber.Ctx) error { return c.SendString(c.Context().RemoteIP().String()) })
	return app
}

func get(t *testing.T, app *fiber.App, path, forwardedFor string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if forwardedFor != "" {
		req.Header.Set(fiber.HeaderXForwardedFor, forwardedFor)
	}
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body)
}

func TestTrustedProxyGivesEachClientItsOwnLimit(t *testing.T) {
	_, peer := get(t, newApp(nil), "/peer", "") // the address the test connection comes from
	app := newApp([]string{peer})

	if code, ip := get(t, app, "/limited", "203.0.113.7, "+peer); code != 200 || ip != "203.0.113.7" {
		t.Fatalf("first client: %d %q", code, ip)
	}
	if code, _ := get(t, app, "/limited", "203.0.113.8"); code != 200 {
		t.Fatalf("a different client behind the same proxy must not share the limit: %d", code)
	}
	if code, _ := get(t, app, "/limited", "203.0.113.7"); code != 429 {
		t.Fatalf("the same client again must be limited: %d", code)
	}
}

func TestUntrustedClientCannotFakeItsAddress(t *testing.T) {
	app := newApp([]string{"10.9.9.9"}) // only this proxy is trusted; the test connection is not it
	if code, _ := get(t, app, "/limited", "203.0.113.7"); code != 200 {
		t.Fatalf("first request: %d", code)
	}
	if code, _ := get(t, app, "/limited", "198.51.100.1"); code != 429 {
		t.Fatalf("a faked X-Forwarded-For must be ignored, so the limit applies: %d", code)
	}
}
