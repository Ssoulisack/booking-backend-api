package postgres

import (
	"backend/internal/adapters/secondary/postgres/migration"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config holds the configuration for PostgreSQL
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	TimeZone string
}

// PostgresRepository interface defines the methods for the PostgreSQL repository
type PostgresRepository interface {
	GetDB() *gorm.DB
	Close() error
}

// PostgresRepository represents the PostgreSQL repository
type postgresRepository struct {
	db *gorm.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(config Config) (PostgresRepository, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode, config.TimeZone,
	)

	gormConfig := &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("❌ failed to connect to database: %w", err)
	}

	// Set connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("❌ failed to get database connection: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Run database migrations
	if err := migration.DatabaseMigrations(db); err != nil {
		return nil, fmt.Errorf("❌ failed to migrate database: %w", err)
	}

	return &postgresRepository{db: db}, nil
}

// GetDB returns the GORM database instance
func (r *postgresRepository) GetDB() *gorm.DB {
	return r.db
}

// Close closes the database connection
func (r *postgresRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
