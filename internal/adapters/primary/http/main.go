package http

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type ServerConfig struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Port         string
	// ProxyHeader is read for the client IP only on requests from TrustedProxies, so clients cannot
	// fake their address to slip past rate limits
	ProxyHeader    string
	TrustedProxies []string
}

type Server struct {
	app    *fiber.App
	config ServerConfig
}

// NewServer creates a new HTTP server
func NewServer(
	config ServerConfig,
) *Server {
	app := fiber.New(fiber.Config{
		ReadTimeout:             config.ReadTimeout,
		WriteTimeout:            config.WriteTimeout,
		ProxyHeader:             config.ProxyHeader,
		EnableTrustedProxyCheck: config.ProxyHeader != "",
		TrustedProxies:          config.TrustedProxies,
		EnableIPValidation:      true, // take the first real IP from a list like "client, proxy"
	})

	// Add middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	server := &Server{
		app:    app,
		config: config,
	}
	return server
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.app.Listen(fmt.Sprintf(":%s", s.config.Port))
}

func (s *Server) App() *fiber.App {
	return s.app
}
