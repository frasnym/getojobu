package version

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShort_returns_build_sha(t *testing.T) {
	assert.NotEmpty(t, Short())
	assert.Regexp(t, `^[0-9a-z.-]+$`, Short())
}

func TestShort_matches_placeholder_file(t *testing.T) {
	data, err := os.ReadFile("version.txt")
	if os.IsNotExist(err) {
		t.Skip("placeholder missing")
	}
	assert.NoError(t, err)
	assert.Equal(t, string(data), Short())
}

func TestShort_is_not_placeholder_after_build(t *testing.T) {
	if Short() == "dev" {
		t.Skip("skipping in local dev; verified inside Docker build")
	}
	assert.NotEqual(t, "dev", Short())
	assert.Regexp(t, `^[0-9a-f]{7}$`, Short())
}
