package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	"github.com/stretchr/testify/assert"
)

func TestReady_returns_503_when_db_unreachable(t *testing.T) {
	// Arrange: a common contract with a nil DB pool.
	common := &common.Contract{}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	// Act.
	Ready(common)(w, req)

	// Assert.
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "expected 503 when DB unreachable")
}
