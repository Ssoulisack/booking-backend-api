// Command migrate creates or updates every table of the database in config.yaml, then stops.
// The API does not migrate on start; run this once after an update that changes the database:
//
//	make migrate                 (uses ./config.yaml)
//
// It only adds what is missing (tables, columns, indexes, rules) and never deletes data.
package main

import (
	"backend/internal/platform/config"
	"backend/internal/platform/postgres"
	"backend/internal/platform/postgres/migration"
	"log"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("❌ Failed to load configuration: %v", err)
	}
	repo, err := postgres.NewPostgresRepository(cfg.Postgres)
	if err != nil {
		log.Fatalf("❌ Failed to connect to the database: %v", err)
	}
	defer repo.Close()

	log.Printf("🔧 Migrating database %q on %s ...", cfg.Postgres.DBName, cfg.Postgres.Host)
	if err := migration.DatabaseMigrations(repo.GetDB()); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}
	log.Println("✅ Database is up to date")
}
