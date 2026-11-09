package main

import (
	"backend/internal/app"
	"backend/internal/http"

	"backend/internal/platform/config"
	"backend/pkg/logger"
	"log"
	"time"
)

func main() {
	// Initialize logger
	logger.NewLogger()
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("❌ Failed to load configuration: %v", err)
	}

	// Initialize HTTP server
	serverConfig := http.ServerConfig{
		Port:           cfg.Server.Port,
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeout) * time.Second,
		ProxyHeader:    cfg.Server.ProxyHeader,
		TrustedProxies: cfg.Server.TrustedProxies,
	}
	server := http.NewServer(
		serverConfig,
	)

	repositories := app.SetupRepositories(cfg)
	defer app.CloseRepositories(repositories)
	services := app.SetupServices(repositories)
	if err := app.Install(services); err != nil {
		log.Fatalf("❌ Failed to prepare the install: %v", err)
	}
	app.KeepTimetablesRolling(services)
	app.KeepTablesSettled(services)
	handlers := app.SetupHandlers(services)
	app.SetupRoutes(server.App(), handlers)
	app.SetupHealth(server.App(), repositories)

	// Start HTTP server
	log.Println("🌟 Server starting on port:", cfg.Server.Port, "🚀")
	if err := server.Start(); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
