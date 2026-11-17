package matching

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppendFailureLine_AppendsAndCaps uses the unexported helper directly so
// the append-only, bounded-history contract is pinned independently of the
// repository call that persists it.
func TestAppendFailureLine_AppendsAndCaps(t *testing.T) {
	existing := "line-1\nline-2\nline-3\nline-4\nline-5\nline-6\nline-7\nline-8\nline-9\nline-10"
	now := time.Date(2026, 11, 17, 10, 30, 0, 0, time.UTC)

	got := appendFailureLine(existing, "Cover letter generation failed", now)

	lines := strings.Split(got, "\n")
	require.Len(t, lines, 10)
	assert.NotContains(t, lines, "line-1")
	assert.Equal(t, "line-2", lines[0])
	assert.Equal(t, "line-10", lines[8])
	assert.Equal(t, "2026-11-17T10:30:00Z — Cover letter generation failed", lines[9])

	parts := strings.SplitN(lines[len(lines)-1], " — ", 2)
	require.Len(t, parts, 2)
	parsed, err := time.Parse(time.RFC3339, parts[0])
	require.NoError(t, err)
	assert.Equal(t, time.UTC, parsed.Location())
}
