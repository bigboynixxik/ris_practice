// Package config reads application settings from environment variables and a dotenv file.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// ErrInvalidConfig indicates an unsupported configuration value.
var ErrInvalidConfig = errors.New("invalid configuration")

// Config contains the settings shared by local and container launches.
type Config struct {
	AppEnv          string        `env:"APP_ENV" envDefault:"local"`
	DatabaseURL     string        `env:"DATABASE_URL" envDefault:"postgres://ris:ris@127.0.0.1:55432/ris?sslmode=disable"`
	SourceURL       string        `env:"SOURCE_URL" envDefault:"https://api.nobelprize.org/v1/laureate.json"`
	JSONFile        string        `env:"JSON_FILE"`
	LogPath         string        `env:"LOG_PATH" envDefault:"logs/ris.log"`
	HTTPTimeout     time.Duration `env:"HTTP_TIMEOUT" envDefault:"30s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"5s"`
}

// LoadConfig reads an optional .env file without replacing process environment values.
func LoadConfig(path string) (*Config, error) {
	if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config.LoadConfig read dotenv: %w", err)
	}
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("config.LoadConfig parse environment: %w", err)
	}
	// Preserve the variable supported by the original command.
	if os.Getenv("DATABASE_URL") == "" && os.Getenv("RIS_DATABASE_URL") != "" {
		cfg.DatabaseURL = os.Getenv("RIS_DATABASE_URL")
	}
	return &cfg, nil
}

// Validate checks the final configuration after command-line overrides.
func (c *Config) Validate() error {
	if c.HTTPTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return fmt.Errorf("config.Validate timeouts must be greater than zero: %w", ErrInvalidConfig)
	}
	if c.AppEnv != "local" && c.AppEnv != "dev" && c.AppEnv != "prod" {
		return fmt.Errorf("config.Validate APP_ENV must be local, dev or prod: %w", ErrInvalidConfig)
	}
	if c.LogPath == "" {
		return fmt.Errorf("config.Validate LOG_PATH must not be empty: %w", ErrInvalidConfig)
	}
	if _, err := pgxpool.ParseConfig(c.DatabaseURL); err != nil {
		return fmt.Errorf("config.Validate database URL: %w", err)
	}
	if c.JSONFile == "" {
		source, err := url.Parse(c.SourceURL)
		if err != nil || (source.Scheme != "http" && source.Scheme != "https") || source.Host == "" {
			return fmt.Errorf("config.Validate SOURCE_URL must be an HTTP or HTTPS URL: %w", ErrInvalidConfig)
		}
	}
	return nil
}
