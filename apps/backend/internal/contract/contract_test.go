package contract

import (
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContract_new_requires_config(t *testing.T) {
	// Act.
	c, _, err := New(nil)

	// Assert.
	require.Error(t, err, "expected error when config is nil")
	assert.Nil(t, c, "expected nil contract on error")
}

func TestContract_new_requires_database_url(t *testing.T) {
	// Arrange.
	cfg := &config.Config{}
	cfg.Server.Addr = ":8080"
	// No DATABASE_URL set: DB init should fail fast.

	// Act.
	c, _, err := New(cfg)

	// Assert.
	require.Error(t, err, "expected error when database URL is missing")
	assert.Nil(t, c, "expected nil contract when DB init fails")
}

func TestContract_new_wires_all_layers(t *testing.T) {
	// Arrange.
	cfg := &config.Config{}
	cfg.Server.Addr = ":8080"
	// Use an unreachable database URL so NewDB creates a pool but Ping fails.
	// This proves DB init happens inside contract.New (fail-fast at startup).
	cfg.DB.URL = "postgres://user:pass@localhost:5432/db?sslmode=disable"

	// Act.
	c, _, err := New(cfg)

	// Assert.
	require.Error(t, err, "expected error when database is unreachable")
	assert.Nil(t, c, "expected nil contract when DB init fails")
}
