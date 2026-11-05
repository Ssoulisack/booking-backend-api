package app

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"backend/internal/platform/config"
	"backend/internal/platform/mailer"
	"backend/internal/platform/postgres"
	"backend/internal/platform/redis"
	"backend/internal/platform/social"
	"backend/internal/platform/storage"
	"log"
)

// Repositories holds all data storage interfaces
type AppRepositories struct {
	// System repositories
	redis    redis.CacheRepository
	mailer   output.Mailer
	postgres postgres.PostgresRepository
	storage  output.FileStorage
	filesDir string // local storage folder served at /files, "" when files live elsewhere
	social   output.SocialVerifier
	// app settings the services need (links in emails) and the company to create on a fresh install
	app      config.AppConfig
	install  config.CompanyConfig
	payments config.PaymentsConfig
	server   config.ServerConfig
	// Domain repositories
	user           output.UserRepository
	company        output.CompanyRepository
	branches       output.BranchRepository
	directory      output.DirectoryRepository
	memberships    output.MembershipRepository
	passwordResets output.PasswordResetRepository
	media          output.MediaRepository
	sports         output.SportRepository
	resources      output.ResourceRepository
	floorItems     output.FloorItemRepository
	coaches        output.CoachRepository
	openingHours   output.OpeningHourRepository
	closures       output.ClosureRepository
	prices         output.PriceRepository
	classSessions  output.ClassSessionRepository
	timetables     output.ClassTimetableRepository
	rules          output.RuleRepository
	bookings       output.BookingRepository
}

// setupRepositories initializes all data storage connections
func SetupRepositories(cfg *config.Config) *AppRepositories {
	// MySQL setup

	// PostgreSQL setup
	postgresRepo, err := postgres.NewPostgresRepository(cfg.Postgres)
	if err != nil {
		log.Fatalf("❌ Failed to initialize PostgreSQL repository: %v", err)
	}

	// Redis setup
	// redisRepo, err := redis.NewRedisRepository(cfg.Redis)
	// if err != nil {
	// 	log.Fatalf("❌ Failed to initialize Redis repository: %v", err)
	// }

	// File storage (local folder or Supabase Storage)
	fileStorage, err := storage.New(cfg.Storage)
	if err != nil {
		log.Fatalf("❌ Failed to initialize file storage: %v", err)
	}

	// Domain repositories
	db := postgresRepo.GetDB()

	return &AppRepositories{
		// redis:          redisRepo,
		mailer:         mailer.NewSystemMailer(cfg.Mailer, cfg.App.Name),
		postgres:       postgresRepo,
		storage:        fileStorage,
		filesDir:       storage.LocalDir(cfg.Storage),
		social:         social.NewVerifier(cfg.Social),
		app:            cfg.App,
		install:        cfg.Company,
		payments:       cfg.Payments,
		server:         cfg.Server,
		user:           postgres.NewUserRepository(db),
		company:        postgres.NewCompanyRepository(db),
		branches:       postgres.NewBranchRepository(db),
		directory:      postgres.NewDirectoryRepository(db),
		memberships:    postgres.NewMembershipRepository(db),
		passwordResets: postgres.NewPasswordResetRepository(db),
		media:          postgres.NewMediaRepository(db),
		sports:         postgres.NewScopedRepository[domain.Sport](db, "name"),
		resources:      postgres.NewScopedRepository[domain.Resource](db, "name"),
		floorItems:     postgres.NewScopedRepository[domain.FloorItem](db, "label"),
		coaches:        postgres.NewCoachRepository(db),
		openingHours:   postgres.NewOpeningHourRepository(db),
		closures:       postgres.NewClosureRepository(db),
		prices:         postgres.NewPriceRepository(db),
		classSessions:  postgres.NewClassSessionRepository(db),
		timetables:     postgres.NewClassTimetableRepository(db),
		rules:          postgres.NewRuleRepository(db),
		bookings:       postgres.NewBookingRepository(db),
	}
}

// closeRepositories gracefully closes all connections
func CloseRepositories(repos *AppRepositories) {
	if repos.redis != nil {
		repos.redis.Close()
	}
	if repos.postgres != nil {
		repos.postgres.Close()
	}
}
