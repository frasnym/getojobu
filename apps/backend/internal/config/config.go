package config

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config holds runtime configuration for the API.
type Config struct {
	Server struct {
		Addr            string
		ReadTimeout     time.Duration
		WriteTimeout    time.Duration
		ShutdownTimeout time.Duration
		SwaggerToken    string // bearer token required to open Swagger UI; empty = no auth (local dev)
	}
	Log         LogConfig
	DB          DBConfig
	Clerk       ClerkConfig
	MachineAuth MachineAuth
	Temporal    TemporalConfig
}

// TemporalConfig holds Temporal orchestrator connection settings.
type TemporalConfig struct {
	Host      string
	Namespace string
	TaskQueue string
}

// MachineKey maps a machine API key to the caller principal it represents.
type MachineKey struct {
	ActorType string
	ActorID   string
}

// MachineAuth holds the mapping of machine API keys to principals.
type MachineAuth struct {
	APIKeys map[string]MachineKey
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level slog.Level
}

// ClerkConfig holds Clerk authentication settings.
type ClerkConfig struct {
	SecretKey         string
	AuthorizedParties []string
	WebhookSecret     string
}

// DBConfig holds database connection settings, populated from DATABASE_URL
// and/or PG* environment variables.
type DBConfig struct {
	URL            string
	Host           string
	Port           string
	Database       string
	User           string
	Password       string
	SSLMode        string
	ChannelBinding string
}

// ErrMissingEnv is returned when a required environment variable is absent.
var ErrMissingEnv = errors.New("required environment variable is missing")

// envString reads name from the environment, returning defaultValue when unset.
func envString(name, defaultValue string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultValue
}

// envDuration reads name as a time.Duration, returning defaultValue when unset or invalid.
func envDuration(name string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(name); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}

// requiredEnv reads name from the environment and errors when it is unset.
func requiredEnv(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", errors.New(name + " is required")
	}
	return v, nil
}

// pgEnv reads name from PG* environment variables.
func pgEnv(name string) string {
	return os.Getenv(name)
}

// envLogLevel reads name as a slog.Level (debug, info, warn, error),
// returning defaultValue when unset or invalid. Matching is case-insensitive.
func envLogLevel(name string, defaultValue slog.Level) slog.Level {
	if v := os.Getenv(name); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "debug":
			return slog.LevelDebug
		case "info":
			return slog.LevelInfo
		case "warn", "warning":
			return slog.LevelWarn
		case "error":
			return slog.LevelError
		}
	}
	return defaultValue
}

// envStringSlice reads name from the environment and splits it by sep,
// returning an empty slice when unset. Empty entries are trimmed and dropped.
func envStringSlice(name, sep string) []string {
	v := os.Getenv(name)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	return result
}

// Load reads configuration from the environment and validates it.
// DATABASE_URL is read as-is; PG* environment variables are stored individually
// so database.New can construct a connection URL from them when DATABASE_URL is
// unset. A .env file is loaded when present (local development); in production
// environment variables are expected to be set directly.
func Load() (*Config, error) {
	cfg := &Config{}

	cfg.Server.Addr = envString("API_SERVER_ADDR", ":8080")
	cfg.Server.ReadTimeout = envDuration("API_SERVER_READ_TIMEOUT", 5*time.Second)
	cfg.Server.WriteTimeout = envDuration("API_SERVER_WRITE_TIMEOUT", 10*time.Second)
	cfg.Server.ShutdownTimeout = envDuration("API_SERVER_SHUTDOWN_TIMEOUT", 15*time.Second)
	cfg.Server.SwaggerToken = envString("SWAGGER_TOKEN", "")

	cfg.Log.Level = envLogLevel("LOG_LEVEL", slog.LevelInfo)

	cfg.DB.URL = os.Getenv("DATABASE_URL")
	cfg.DB.Host = pgEnv("PG_HOST")
	cfg.DB.Port = pgEnv("PG_PORT")
	cfg.DB.Database = pgEnv("PG_DATABASE")
	cfg.DB.User = pgEnv("PG_USER")
	cfg.DB.Password = pgEnv("PG_PASSWORD")
	cfg.DB.SSLMode = pgEnv("PG_SSL_MODE")
	cfg.DB.ChannelBinding = pgEnv("PG_CHANNEL_BINDING")

	cfg.Clerk.SecretKey = os.Getenv("CLERK_SECRET_KEY")
	cfg.Clerk.AuthorizedParties = envStringSlice("CLERK_AUTHORIZED_PARTIES", ",")
	cfg.Clerk.WebhookSecret = os.Getenv("CLERK_WEBHOOK_SECRET")

	cfg.MachineAuth.APIKeys = map[string]MachineKey{}
	if v := os.Getenv("CRON_API_KEY"); v != "" {
		cfg.MachineAuth.APIKeys[v] = MachineKey{ActorType: "cron", ActorID: "cron-scrape"}
	}
	if v := os.Getenv("HERMES_API_KEY"); v != "" {
		cfg.MachineAuth.APIKeys[v] = MachineKey{ActorType: "agent", ActorID: "hermes"}
	}
	if v := os.Getenv("PLAYWRIGHT_API_KEY"); v != "" {
		cfg.MachineAuth.APIKeys[v] = MachineKey{ActorType: "worker", ActorID: "playwright-worker"}
	}
	if v := os.Getenv("WORKER_API_KEY"); v != "" {
		cfg.MachineAuth.APIKeys[v] = MachineKey{ActorType: "worker", ActorID: "getojobu-worker"}
	}

	cfg.Temporal.Host = envString("TEMPORAL_HOST", "localhost:7233")
	cfg.Temporal.Namespace = envString("TEMPORAL_NAMESPACE", "default")
	cfg.Temporal.TaskQueue = envString("TEMPORAL_TASK_QUEUE_JOB_ANALYZE", "job-evaluation")

	return cfg, nil
}
