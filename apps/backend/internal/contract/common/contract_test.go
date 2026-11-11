package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_nil_config_returns_error(t *testing.T) {
	contract, stopper, err := New(nil)
	require.Error(t, err)
	assert.Nil(t, contract)
	assert.Nil(t, stopper)
	assert.EqualError(t, err, "config is required")
}
