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

func TestHealth_returns_200_with_status_ok(t *testing.T) {
	// Arrange.
	h := New(&contract.Contract{Common: &common.Contract{}})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Act.
	err := h.Health(w, req)

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, http.StatusOK, w.Code, "expected 200")
	assert.Equal(t, `{"status":"ok"}`+"\n", w.Body.String(), "expected {\"status\":\"ok\"}")
}
