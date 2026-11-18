package utils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripJSONKeys(t *testing.T) {
	t.Run("removes the requested keys and keeps the rest", func(t *testing.T) {
		raw := json.RawMessage(`{"id":7,"family":"technology","is_active":true}`)

		var data map[string]any
		require.NoError(t, json.Unmarshal(StripJSONKeys(raw, "id", "is_active"), &data))
		assert.Equal(t, "technology", data["family"])
		assert.NotContains(t, data, "id")
		assert.NotContains(t, data, "is_active")
	})

	t.Run("does not mutate the stored value", func(t *testing.T) {
		raw := json.RawMessage(`{"metadata":{"model":"x"}}`)
		StripJSONKeys(raw, "metadata")

		var data map[string]any
		require.NoError(t, json.Unmarshal(raw, &data))
		assert.Contains(t, data, "metadata")
	})

	t.Run("keeps JSON when none of the keys are present", func(t *testing.T) {
		raw := json.RawMessage(`{"family":"technology"}`)
		assert.JSONEq(t, string(raw), string(StripJSONKeys(raw, "id", "metadata")))
	})

	t.Run("keeps JSON when no keys are given", func(t *testing.T) {
		raw := json.RawMessage(`{"family":"technology"}`)
		assert.Equal(t, raw, StripJSONKeys(raw))
	})

	t.Run("keeps invalid or non-object JSON as-is", func(t *testing.T) {
		for _, raw := range []json.RawMessage{nil, json.RawMessage(`[]`), json.RawMessage(`not json`)} {
			assert.Equal(t, raw, StripJSONKeys(raw, "metadata"))
		}
	})
}
