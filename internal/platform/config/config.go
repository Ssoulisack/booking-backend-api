package config

import (
	"backend/internal/platform/mailer"
	"backend/internal/platform/postgres"
	"backend/internal/platform/redis"
	"backend/internal/platform/social"
	"backend/internal/platform/storage"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// "mega-erp-backend-svc/internal/adapters/secondary/postgres"
// "mega-erp-backend-svc/internal/adapters/secondary/redis"

// Config holds the application configuration
type Config struct {
	App      AppConfig
	Server   ServerConfig
	Postgres postgres.Config
	Redis    redis.Config
	JWT      JWTConfig
	Storage  storage.Config
	Social   social.Config
	Payments PaymentsConfig
	Mailer   mailer.Config
	Company  CompanyConfig
	Telegram TelegramConfig
}

// TelegramConfig: how the server reaches Telegram and seals what branches store. A branch's own bot token and group
// are NOT here: owners connect them on the website. EncryptionKey is the secret the stored token is encrypted with;
// left empty, the jwt.secret is used. Changing it later makes stored tokens unreadable (they are entered again).
type TelegramConfig struct {
	APIBase       string // default https://api.telegram.org (a stand-in address in tests)
	EncryptionKey string
}

// PaymentsConfig: online payment gateways (config.yaml "payments"). A gateway with no keys is off and the
// booking pages keep the QR / transfer / WhatsApp way. Keys stay on the server and are never logged.
type PaymentsConfig struct {
	PhaJay PhaJayConfig
	Mock   MockConfig
}

// MockConfig: the built-in stand-in gateway used when server.env is dev
type MockConfig struct {
	// DailyLimit refuses more than this many payments a day, to try the "limit reached" message (the real
	// test account allows 20); 0 = no limit
	DailyLimit int
}

// PhaJayConfig: https://payment-doc.phajay.co/v1 (keys in the portal https://portal.phajay.co).
// Before KYC use TestKey for calls and DevSecretKey for webhooks; after approval SecretKey does both.
// Keys can contain "$": write them in double quotes in YAML and check the length in the start-up log.
type PhaJayConfig struct {
	BaseURL      string // default https://payment-gateway.phajay.co
	TestKey      string // calls to the API before KYC
	DevSecretKey string // webhook signing before KYC
	SecretKey    string // production: calls and webhook signing
	// TestAmountDivisor makes tests possible before KYC, when the gateway only takes 1 to 999 kip: the
	// amount sent is the real amount divided by this (100000: 100,000 kip is sent as 1), and the gateway's
	// amounts are multiplied back. Only used while no production SecretKey is set; 0 = off.
	TestAmountDivisor int
}

// CompanyConfig names the client this install belongs to. On the first start the company and its
// owner are created from it; afterwards the owner changes everything in the app (only an empty
// database reads it again). OwnerPassword is for local development: when empty, the owner gets an
// email to choose a password.
type CompanyConfig struct {
	Name          string
	OwnerName     string
	OwnerEmail    string
	OwnerPassword string
}

// AppConfig holds install-wide settings. FrontendURL is the base of links in emails
// (invites, password resets), e.g. https://booking.example.com
type AppConfig struct {
	Name        string
	FrontendURL string
}

// ServerConfig holds the HTTP server configuration
type ServerConfig struct {
	// Env decides which payment gateway is used: dev = the built-in mock (PhaJay is never called), uat =
	// PhaJay's test account, prod = PhaJay with the production key (after KYC). Empty means dev.
	Env          string
	Port         string
	ReadTimeout  int
	WriteTimeout int
	// Behind a load balancer: the header carrying the client IP (e.g. X-Forwarded-For) and the
	// proxy addresses allowed to set it. Empty = clients connect directly.
	ProxyHeader    string
	TrustedProxies []string
	// Abuse limits for the public booking page. 0 = the default of the environment: 10 bookings per IP per hour and 3
	// waiting bookings per phone in uat and prod, practically no limit in dev (every test comes from one address).
	// PublicBookingsPerHour: new bookings from one IP address per hour (a venue's shared Wi-Fi counts as one address).
	// OpenWaitingPerPhone: bookings one phone number may have waiting for approval and unpaid at the same time.
	PublicBookingsPerHour int
	OpenWaitingPerPhone   int
}

// Environments (server.env)
const (
	EnvDev  = "dev"
	EnvUAT  = "uat"
	EnvProd = "prod"
)

// Mode is server.env in lower case; dev when it is empty
func (s ServerConfig) Mode() string {
	if e := strings.ToLower(strings.TrimSpace(s.Env)); e != "" {
		return e
	}
	return EnvDev
}

// JWTConfig holds the JWT configuration
type JWTConfig struct {
	Secret        string
	RefreshSecret string
}

var JWTGlobal JWTConfig

// LoadConfig loads the application configuration from environment variables and config files
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Set default values
	setDefaults()

	// Read from config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("❌ failed to read config file: %w", err)
		}
		// Config file not found, will use defaults and environment variables
	}

	// Read from environment variables
	viper.SetEnvPrefix("APP")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("❌ failed to unmarshal config: %w", err)
	}
	// Postgres settings live under database.postgres, not at the top level.
	// Read key by key so APP_DATABASE_POSTGRES_* environment overrides still apply.
	config.Postgres = postgres.Config{
		Host:     viper.GetString("database.postgres.host"),
		Port:     viper.GetString("database.postgres.port"),
		User:     viper.GetString("database.postgres.user"),
		Password: viper.GetString("database.postgres.password"),
		DBName:   viper.GetString("database.postgres.dbname"),
		SSLMode:  viper.GetString("database.postgres.sslmode"),
		TimeZone: viper.GetString("database.postgres.timezone"),
	}
	switch config.Server.Mode() {
	case EnvDev, EnvUAT, EnvProd:
	default:
		return nil, fmt.Errorf("❌ server.env %q is not one of dev, uat, prod", config.Server.Env)
	}
	JWTGlobal = config.JWT
	return &config, nil
}

// setDefaults sets the default values for the configuration
func setDefaults() {
	// App defaults
	viper.SetDefault("app.name", "Sport Booking")
	viper.SetDefault("app.frontendUrl", "http://localhost:5173")

	// Server defaults
	viper.SetDefault("server.env", "dev")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.readTimeout", 10)
	viper.SetDefault("server.writeTimeout", 10)

	// Postgres defaults
	viper.SetDefault("database.postgres.host", "localhost")
	viper.SetDefault("database.postgres.port", "5432")
	viper.SetDefault("database.postgres.user", "root")
	viper.SetDefault("database.postgres.password", "root")
	viper.SetDefault("database.postgres.dbname", "local")
	viper.SetDefault("database.postgres.sslmode", "disable")
	viper.SetDefault("database.postgres.timezone", "UTC")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.enabled", false) // the Redis cache of rarely-changing reads: off unless switched on
	viper.SetDefault("redis.ttlSeconds", 600)

	// JWT defaults
	viper.SetDefault("jwt.secret", "your-secret-key")
	viper.SetDefault("jwt.refreshSecret", "your-refresh-secret-key")

	// Storage defaults: files in a folder on this machine (driver r2 for Cloudflare R2, supabase for
	// Supabase Storage). Every key has a default so APP_STORAGE_* environment overrides apply.
	viper.SetDefault("storage.driver", "local")
	viper.SetDefault("storage.local.path", "./data/uploads")
	viper.SetDefault("storage.local.publicBaseUrl", "")
	viper.SetDefault("storage.r2.endpoint", "")
	viper.SetDefault("storage.r2.region", "auto")
	viper.SetDefault("storage.r2.bucket", "")
	viper.SetDefault("storage.r2.accessKey", "")
	viper.SetDefault("storage.r2.secretKey", "")
	viper.SetDefault("storage.r2.publicBaseUrl", "")
	viper.SetDefault("storage.supabase.projectUrl", "")
	viper.SetDefault("storage.supabase.bucket", "")
	viper.SetDefault("storage.supabase.serviceKey", "")
	viper.SetDefault("storage.supabase.publicBaseUrl", "")

	// Social sign-in (Google, Facebook): a provider with empty credentials is switched off
	viper.SetDefault("social.googleClientId", "")
	viper.SetDefault("social.facebookAppId", "")
	viper.SetDefault("social.facebookAppSecret", "")

	// Payment gateways: empty keys switch a gateway off
	viper.SetDefault("payments.phajay.baseUrl", "https://payment-gateway.phajay.co")
	viper.SetDefault("payments.phajay.testKey", "")
	viper.SetDefault("payments.phajay.devSecretKey", "")
	viper.SetDefault("payments.phajay.secretKey", "")
	viper.SetDefault("payments.phajay.testAmountDivisor", 0)
	viper.SetDefault("payments.mock.dailyLimit", 0)

	// Telegram: the address, and the key stored bot tokens are sealed with (empty: the jwt secret)
	viper.SetDefault("telegram.apiBase", "https://api.telegram.org")
	viper.SetDefault("telegram.encryptionKey", "")

	// Mailer defaults
	viper.SetDefault("mailer.host", "smtp.example.com")
	viper.SetDefault("mailer.username", "username")
	viper.SetDefault("mailer.password", "password")
	viper.SetDefault("mailer.port", 000)
	viper.SetDefault("mailer.from", "from")
}
