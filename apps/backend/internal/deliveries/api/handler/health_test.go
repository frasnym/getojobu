package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealth_returns_200_with_status_ok(t *testing.T) {
	// Arrange.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Act.
	Health()(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code, "expected 200")
	assert.Equal(t, `{"status":"ok"}`+"\n", w.Body.String(), "expected {\"status\":\"ok\"}")
}
