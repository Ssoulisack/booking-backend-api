package app

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"backend/internal/platform/cache"
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
	app       config.AppConfig
	install   config.CompanyConfig
	payments  config.PaymentsConfig
	server    config.ServerConfig
	telegram  config.TelegramConfig
	cache     *cache.Cache // Redis cache of rarely-changing reads; off unless redis.enabled
	jwtSecret string
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
	bookingReports output.BookingReportRepository
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

	// Cache of rarely-changing reads (off unless redis.enabled): cleared now, and a Redis error means "read the database"
	reads := cache.New(cfg.Redis)

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
		telegram:       cfg.Telegram,
		cache:          reads,
		jwtSecret:      cfg.JWT.Secret,
		user:           postgres.NewUserRepository(db),
		company:        cache.WrapCompany(postgres.NewCompanyRepository(db), reads),
		branches:       cache.WrapBranches(postgres.NewBranchRepository(db), reads),
		directory:      cache.WrapDirectory(postgres.NewDirectoryRepository(db), reads),
		memberships:    postgres.NewMembershipRepository(db),
		passwordResets: postgres.NewPasswordResetRepository(db),
		media:          postgres.NewMediaRepository(db),
		// a sport change also changes the public list of places, which shows each branch's sports
		sports: cache.WrapScoped(postgres.NewScopedRepository[domain.Sport](db, "name"), reads, "sports",
			func(m *domain.Sport) uint { return m.BranchID }, cache.ScopeDirectory),
		resources: cache.WrapScoped(postgres.NewScopedRepository[domain.Resource](db, "name"), reads, "resources",
			func(m *domain.Resource) uint { return m.BranchID }),
		floorItems: cache.WrapScoped(postgres.NewScopedRepository[domain.FloorItem](db, "label"), reads, "floor",
			func(m *domain.FloorItem) uint { return m.BranchID }),
		coaches:        cache.WrapCoaches(postgres.NewCoachRepository(db), reads),
		openingHours:   cache.WrapHours(postgres.NewOpeningHourRepository(db), reads),
		closures:       cache.WrapClosures(postgres.NewClosureRepository(db), reads),
		prices:         cache.WrapPrices(postgres.NewPriceRepository(db), reads),
		classSessions:  postgres.NewClassSessionRepository(db),
		timetables:     postgres.NewClassTimetableRepository(db),
		rules:          cache.WrapRules(postgres.NewRuleRepository(db), reads),
		bookings:       postgres.NewBookingRepository(db),
		bookingReports: postgres.NewBookingReportRepository(db),
	}
}

// closeRepositories gracefully closes all connections
func CloseRepositories(repos *AppRepositories) {
	repos.cache.Close()
	if repos.redis != nil {
		repos.redis.Close()
	}
	if repos.postgres != nil {
		repos.postgres.Close()
	}
}
