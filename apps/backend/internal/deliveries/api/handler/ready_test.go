package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReady_returns_503_when_db_unreachable(t *testing.T) {
	// Arrange: a common contract with a nil DB pool.
	h := New(&contract.Contract{Common: &common.Contract{}})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	// Act.
	err := h.Ready(w, req)

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "expected 503 when DB unreachable")
}
