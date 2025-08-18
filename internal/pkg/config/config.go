package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// -----------------------------------------------------------------------------
// Environment variable configuration guidelines:
// - required: Values that differ between environments (port, DB connection, etc.), security settings
// - default: Values common across all environments (timezone, timeout, etc.), standard settings
// -----------------------------------------------------------------------------

type Config struct {
	Server ServerConfig
	DB     DBConfig
	CORS   CORSConfig
	Log    LogConfig
	JWT    JWTConfig
}

type ServerConfig struct {
	Port string `envconfig:"PORT" required:"true"`
}

type DBConfig struct {
	Host     string `envconfig:"DB_HOST" default:"localhost"`
	Port     string `envconfig:"DB_PORT" default:"5432"`
	User     string `envconfig:"DB_USER" required:"true"`
	Password string `envconfig:"DB_PASSWORD" required:"true"`
	DBName   string `envconfig:"DB_NAME" required:"true"`
	SSLMode  string `envconfig:"DB_SSL_MODE" default:"disable"`
	TimeZone string `envconfig:"DB_TIMEZONE" default:"Asia/Tokyo"`
}

type CORSConfig struct {
	AllowOrigins     []string      `envconfig:"CORS_ALLOW_ORIGINS" default:"http://localhost:3000,http://localhost:8080"`
	AllowMethods     []string      `envconfig:"CORS_ALLOW_METHODS" default:"GET,POST,PUT,PATCH,DELETE,OPTIONS"`
	AllowHeaders     []string      `envconfig:"CORS_ALLOW_HEADERS" default:"Origin,Content-Type,Accept,Authorization"`
	ExposeHeaders    []string      `envconfig:"CORS_EXPOSE_HEADERS" default:"Content-Length"`
	AllowCredentials bool          `envconfig:"CORS_ALLOW_CREDENTIALS" default:"true"`
	MaxAge           time.Duration `envconfig:"CORS_MAX_AGE" default:"12h"`
}

type LogConfig struct {
	Level          string `envconfig:"LOG_LEVEL" default:"info"`
	TimeZone       string `envconfig:"LOG_TIMEZONE" default:"Asia/Tokyo"`
	TimeFormat     string `envconfig:"LOG_TIME_FORMAT" default:"2006-01-02 15:04:05.000"`
	TimeZoneOffset int    `envconfig:"LOG_TIMEZONE_OFFSET" default:"32400"` // 9*60*60
}

type JWTConfig struct {
	Secret   string `envconfig:"JWT_SECRET" required:"true"`
	Duration string `envconfig:"JWT_DURATION" default:"24h"`
}

func (c *DBConfig) BuildDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&timezone=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode, c.TimeZone,
	)
}

func LoadConfig() (Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("failed to process env config: %w", err)
	}
	return cfg, nil
}

func NewTestConfig() Config {
	return Config{
		Server: ServerConfig{
			Port: "8889", // Test port
		},
		DB: DBConfig{
			Host:     "localhost",
			Port:     "15433", // Test DB port
			User:     "test",
			Password: "test",
			DBName:   "test_db",
			SSLMode:  "disable",
			TimeZone: "Asia/Tokyo",
		},
		Log: LogConfig{
			Level:          "error", // Error level only for tests
			TimeZone:       "Asia/Tokyo",
			TimeFormat:     "2006-01-02 15:04:05.000",
			TimeZoneOffset: 32400,
		},
	}
}
