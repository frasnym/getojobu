package contract

import (
	"context"
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

func TestContract_new_wires_all_layers(t *testing.T) {
	// Arrange.
	cfg := &config.Config{}
	cfg.Server.Addr = ":8080"

	// Act.
	c, stopper, err := New(cfg)

	// Assert.
	require.NoError(t, err, "unexpected error")
	require.NotNil(t, c, "expected non-nil contract")
	assert.Equal(t, cfg, c.Cfg, "expected Cfg to be set")
	assert.NotNil(t, c.Usecase, "expected Usecase to be set")
	assert.NotNil(t, c.Repository, "expected Repository to be set")
	assert.NotNil(t, c.Common, "expected Common to be set")
	assert.Nil(t, c.Common.DB, "expected Common.DB to be nil (DB init is separate from contract wiring)")

	// Cleanup via stopper.
	assert.NoError(t, stopper(context.Background()), "unexpected stopper error")
}
