package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonContract_health_returns_error_when_nil(t *testing.T) {
	// Act.
	var c *Contract
	err := c.Health(nil)

	// Assert.
	require.Error(t, err, "expected error for nil common contract")
	assert.EqualError(t, err, "database not initialized")
}
