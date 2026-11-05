package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestID_generates_and_echoes_header(t *testing.T) {
	// Arrange.
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Context().Value("unused") == nil {
			// noop
		}
		_ = r.Context()
	})

	handler := RequestID(next)

	// Act: no inbound request id.
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Assert: response header set and handler called.
	assert.NotEmpty(t, w.Header().Get("X-Request-Id"), "expected X-Request-Id response header to be set")
	assert.True(t, called, "expected next handler to be called")

	// Act: echo inbound request id.
	req = httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("X-Request-Id", "req-abc123")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "req-abc123", w.Header().Get("X-Request-Id"), "expected echoed request id req-abc123")
}

func TestRecovery_captures_panic_and_returns_500(t *testing.T) {
	// Arrange.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	// Act.
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	Recovery(next).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusInternalServerError, w.Code, "expected 500 after panic")
}
