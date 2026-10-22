package mysql

import (
	"backend/internal/adapters/secondary/mysql/migration"
	"fmt"

	"time"

	"gorm.io/driver/mysql"
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

// MySql Repository interface defines the methods for the MySQL repository
type MySQLRepository interface {
	GetDB() *gorm.DB
	Close() error
}

// PostgresRepository represents the PostgreSQL repository
type mySQLRepository struct {
	db *gorm.DB
}

// NewMySQLRepository creates a new MySQL repository
func NewMySQLRepository(config Config) (MySQLRepository, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local",
		config.User, config.Password, config.Host, config.Port, config.DBName,
	)

	gormConfig := &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
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

	return &mySQLRepository{db: db}, nil
}

// GetDB returns the GORM database instance
func (r *mySQLRepository) GetDB() *gorm.DB {
	return r.db
}

// Close closes the database connection
func (r *mySQLRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
