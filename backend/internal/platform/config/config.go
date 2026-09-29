// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Supported values of APP_ENV.
const (
	EnvDev     = "dev"
	EnvStaging = "staging"
	EnvProd    = "prod"
)

type Config struct {
	// AppEnv is one of dev, staging, prod. Only "dev" enables dev-only
	// conveniences such as the "Authorization: dev <id>" bypass.
	AppEnv   string
	HTTPAddr string
	LogLevel string

	DatabaseURL string

	// BotToken is required outside dev: it is both the bot credential and
	// the key used to validate Telegram initData.
	BotToken string
	// WebAppURL is the public HTTPS URL of the Mini App, used for the
	// "Open" button in the bot's /start reply. Optional.
	WebAppURL string

	ReminderInterval time.Duration
	ShutdownTimeout  time.Duration
}

func (c Config) IsDev() bool { return c.AppEnv == EnvDev }

// Load reads configuration from the environment and validates it.
func Load() (Config, error) {
	var errs []error

	cfg := Config{
		AppEnv:      strings.TrimSpace(getenv("APP_ENV", EnvProd)),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BotToken:    os.Getenv("BOT_TOKEN"),
		WebAppURL:   os.Getenv("WEBAPP_URL"),
	}

	switch cfg.AppEnv {
	case EnvDev, EnvStaging, EnvProd:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be one of dev, staging, prod; got %q", cfg.AppEnv))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if cfg.BotToken == "" && !cfg.IsDev() {
		errs = append(errs, errors.New("BOT_TOKEN is required when APP_ENV is not dev"))
	}
	if cfg.WebAppURL != "" && !strings.HasPrefix(cfg.WebAppURL, "https://") {
		errs = append(errs, errors.New("WEBAPP_URL must start with https:// (Telegram requirement)"))
	}

	var err error
	if cfg.ReminderInterval, err = durationEnv("REMINDER_INTERVAL", time.Minute); err != nil {
		errs = append(errs, err)
	}
	if cfg.ShutdownTimeout, err = durationEnv("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		errs = append(errs, err)
	}

	return cfg, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration like 30s or 1m; got %q", key, v)
	}
	return d, nil
}
