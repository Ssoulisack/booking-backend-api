package application

import (
	"backend/internal/adapters/secondary/mailer"
	"backend/internal/adapters/secondary/minio"
	"backend/internal/adapters/secondary/postgres"
	"backend/internal/adapters/secondary/redis"
	"backend/internal/adapters/secondary/social"
	"backend/internal/adapters/secondary/storage"
	"backend/internal/config"
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"log"
)

// Repositories holds all data storage interfaces
type AppRepositories struct {
	// System repositories
	// mysql    mysql.MySQLRepository
	redis    redis.CacheRepository
	minio    minio.MinIORepository
	mailer   output.Mailer
	postgres postgres.PostgresRepository
	storage  output.FileStorage
	social   output.SocialVerifier
	// app settings the services need (links in emails) and the company to create on a fresh install
	app     config.AppConfig
	install config.CompanyConfig
	// Domain repositories
	user           output.UserRepository
	company        output.CompanyRepository
	branches       output.BranchRepository
	directory      output.DirectoryRepository
	memberships    output.MembershipRepository
	invitations    output.InvitationRepository
	passwordResets output.PasswordResetRepository
	socialIDs      output.SocialIdentityRepository
	media          output.MediaRepository
	sports         output.SportRepository
	resources      output.ResourceRepository
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
	// mysqlRepo, err := mysql.NewMySQLRepository(cfg.Database)
	// if err != nil {
	// 	log.Fatalf("❌ Failed to initialize MySQL repository: %v", err)
	// }

	// PostgreSQL setup
	postgresRepo, err := postgres.NewPostgresRepository(cfg.Postgres)
	if err != nil {
		log.Fatalf("❌ Failed to initialize PostgreSQL repository: %v", err)
	}

	// Redis setup
	redisRepo, err := redis.NewRedisRepository(cfg.Redis)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Redis repository: %v", err)
	}

	// MinIO setup
	minioRepo, err := minio.NewMinIORepository(cfg.Minio)
	if err != nil {
		log.Fatalf("❌ Failed to initialize MinIO repository: %v", err)
	}

	// Domain repositories
	db := postgresRepo.GetDB()

	return &AppRepositories{
		// mysql:  mysqlRepo,
		redis:          redisRepo,
		minio:          minioRepo,
		mailer:         mailer.NewSystemMailer(cfg.Mailer, cfg.App.Name),
		postgres:       postgresRepo,
		storage:        storage.NewSupabaseStorage(cfg.Storage),
		social:         social.NewVerifier(cfg.Social),
		app:            cfg.App,
		install:        cfg.Company,
		user:           postgres.NewUserRepository(db),
		company:        postgres.NewCompanyRepository(db),
		branches:       postgres.NewBranchRepository(db),
		directory:      postgres.NewDirectoryRepository(db),
		memberships:    postgres.NewMembershipRepository(db),
		invitations:    postgres.NewInvitationRepository(db),
		passwordResets: postgres.NewPasswordResetRepository(db),
		socialIDs:      postgres.NewSocialIdentityRepository(db),
		media:          postgres.NewMediaRepository(db),
		sports:         postgres.NewScopedRepository[domain.Sport](db, "name"),
		resources:      postgres.NewScopedRepository[domain.Resource](db, "name"),
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
	// if repos.mysql != nil {
	// 	repos.mysql.Close()
	// }
	if repos.redis != nil {
		repos.redis.Close()
	}
	if repos.postgres != nil {
		repos.postgres.Close()
	}
}
