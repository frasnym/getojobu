package config

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_applies_default_ports_and_timeouts(t *testing.T) {
	// Arrange.
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=require")
	defer os.Unsetenv("DATABASE_URL")
	os.Unsetenv("API_SERVER_ADDR")
	os.Unsetenv("API_SERVER_READ_TIMEOUT")
	os.Unsetenv("API_SERVER_WRITE_TIMEOUT")
	os.Unsetenv("API_SERVER_SHUTDOWN_TIMEOUT")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, ":8080", cfg.Server.Addr, "expected default addr :8080")
	assert.Equal(t, 5*time.Second, cfg.Server.ReadTimeout, "expected default read timeout 5s")
	assert.Equal(t, 10*time.Second, cfg.Server.WriteTimeout, "expected default write timeout 10s")
	assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout, "expected default shutdown timeout 15s")
	assert.Equal(t, "postgres://user:pass@localhost:5432/db?sslmode=require", cfg.DB.URL, "expected DATABASE_URL to be passed through")
}

func TestLoad_parses_duration_overrides(t *testing.T) {
	// Arrange.
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
	os.Setenv("API_SERVER_READ_TIMEOUT", "30s")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("API_SERVER_READ_TIMEOUT")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout, "expected read timeout 30s")
}

func TestLoad_passes_through_pg_env_fields(t *testing.T) {
	// Arrange: unset DATABASE_URL, set PG* vars.
	os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("DATABASE_URL")
	os.Setenv("PG_HOST", "db.example.com")
	os.Setenv("PG_PORT", "5432")
	os.Setenv("PG_DATABASE", "mydb")
	os.Setenv("PG_USER", "myuser")
	os.Setenv("PG_PASSWORD", "mypass")
	os.Setenv("PG_SSL_MODE", "require")
	os.Setenv("PG_CHANNEL_BINDING", "prefer")
	defer os.Unsetenv("PG_HOST")
	defer os.Unsetenv("PG_PORT")
	defer os.Unsetenv("PG_DATABASE")
	defer os.Unsetenv("PG_USER")
	defer os.Unsetenv("PG_PASSWORD")
	defer os.Unsetenv("PG_SSL_MODE")
	defer os.Unsetenv("PG_CHANNEL_BINDING")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, "", cfg.DB.URL, "expected DATABASE_URL to be empty when unset")
	assert.Equal(t, "db.example.com", cfg.DB.Host, "expected PG_HOST")
	assert.Equal(t, "5432", cfg.DB.Port, "expected PG_PORT")
	assert.Equal(t, "mydb", cfg.DB.Database, "expected PG_DATABASE")
	assert.Equal(t, "myuser", cfg.DB.User, "expected PG_USER")
	assert.Equal(t, "mypass", cfg.DB.Password, "expected PG_PASSWORD")
	assert.Equal(t, "require", cfg.DB.SSLMode, "expected PG_SSL_MODE")
	assert.Equal(t, "prefer", cfg.DB.ChannelBinding, "expected PG_CHANNEL_BINDING")
}

func TestLoad_database_url_takes_precedence_over_pg_env(t *testing.T) {
	// Arrange: set both DATABASE_URL and PG* vars.
	os.Setenv("DATABASE_URL", "postgres://explicit:pass@explicit.host:9999/explicitdb?sslmode=disable")
	os.Setenv("PG_HOST", "pg.example.com")
	os.Setenv("PG_DATABASE", "pgdb")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("PG_HOST")
	defer os.Unsetenv("PG_DATABASE")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, "postgres://explicit:pass@explicit.host:9999/explicitdb?sslmode=disable", cfg.DB.URL, "expected DATABASE_URL to take precedence")
}

func TestLoad_clerk_config(t *testing.T) {
	// Arrange.
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=require")
	os.Setenv("CLERK_SECRET_KEY", "sk_test_1234567890")
	os.Setenv("CLERK_AUTHORIZED_PARTIES", "http://localhost:3000, https://app.getojobu.com ,")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("CLERK_SECRET_KEY")
	defer os.Unsetenv("CLERK_AUTHORIZED_PARTIES")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, "sk_test_1234567890", cfg.Clerk.SecretKey, "expected Clerk secret key")
	assert.Equal(t, []string{"http://localhost:3000", "https://app.getojobu.com"}, cfg.Clerk.AuthorizedParties, "expected Clerk authorized parties")
}

func TestLoad_log_level_defaults_to_info(t *testing.T) {
	// Arrange.
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=require")
	os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("DATABASE_URL")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, slog.LevelInfo, cfg.Log.Level, "expected default log level info")
}

func TestLoad_log_level_override(t *testing.T) {
	// Arrange.
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=require")
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("LOG_LEVEL")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, slog.LevelDebug, cfg.Log.Level, "expected log level debug")
}

func TestLoad_log_level_case_insensitive_and_invalid_falls_back_to_default(t *testing.T) {
	// Arrange.
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=require")
	os.Setenv("LOG_LEVEL", "WARN")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("LOG_LEVEL")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, slog.LevelWarn, cfg.Log.Level, "expected log level warn case-insensitive")

	// Act: invalid level falls back to default.
	os.Setenv("LOG_LEVEL", "not-a-level")
	cfg2, err2 := Load()

	require.NoError(t, err2, "unexpected error")
	assert.Equal(t, slog.LevelInfo, cfg2.Log.Level, "expected default log level info for invalid input")
}

func TestLoad_clerk_config_empty_when_unset(t *testing.T) {
	// Arrange.
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=require")
	os.Unsetenv("CLERK_SECRET_KEY")
	os.Unsetenv("CLERK_AUTHORIZED_PARTIES")
	defer os.Unsetenv("DATABASE_URL")

	// Act.
	cfg, err := Load()

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Empty(t, cfg.Clerk.SecretKey, "expected empty Clerk secret key when unset")
	assert.Empty(t, cfg.Clerk.AuthorizedParties, "expected empty Clerk authorized parties when unset")
}
