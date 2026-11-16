package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// captureStdout runs fn and returns everything fn wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	require.NoError(t, w.Close())
	os.Stdout = old
	return <-done
}

// parseLogEntry decodes the single JSON log line written by captureStdout.
func parseLogEntry(t *testing.T, out string) map[string]any {
	t.Helper()
	line := strings.TrimSpace(out)
	require.NotEmpty(t, line, "expected a log line")

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &entry), "expected valid JSON log line, got %q", line)
	return entry
}

func TestLogging_logs_request_and_response_body(t *testing.T) {
	// Arrange.
	const reqPayload = `{"slug":"acme"}`
	const resPayload = `{"success":false}`

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, reqPayload, string(body), "handler must still read the full request body")

		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(resPayload))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/bulk", strings.NewReader(reqPayload))

	// Act.
	var w *httptest.ResponseRecorder
	out := captureStdout(t, func() {
		w = httptest.NewRecorder()
		Logging(next).ServeHTTP(w, req)
	})

	// Assert.
	entry := parseLogEntry(t, out)
	assert.Equal(t, reqPayload, entry["req_body"])
	assert.Equal(t, float64(len(reqPayload)), entry["req_body_bytes"])
	assert.Equal(t, false, entry["req_body_truncated"])
	assert.Equal(t, resPayload, entry["res_body"])
	assert.Equal(t, float64(len(resPayload)), entry["res_body_bytes"])
	assert.Equal(t, false, entry["res_body_truncated"])
	assert.Equal(t, resPayload, w.Body.String())
}

func TestLogging_truncates_bodies_over_limit(t *testing.T) {
	// Arrange.
	const bodySize = 10000
	handlerSaw := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		handlerSaw = len(body)
		_, _ = w.Write([]byte(strings.Repeat("b", bodySize)))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/bulk", strings.NewReader(strings.Repeat("a", bodySize)))

	// Act.
	var w *httptest.ResponseRecorder
	out := captureStdout(t, func() {
		w = httptest.NewRecorder()
		Logging(next).ServeHTTP(w, req)
	})

	// Assert: logged bodies are capped, totals and flags still recorded.
	entry := parseLogEntry(t, out)
	assert.Len(t, entry["req_body"], maxLogBody)
	assert.Equal(t, true, entry["req_body_truncated"])
	assert.Equal(t, float64(bodySize), entry["req_body_bytes"])
	assert.Len(t, entry["res_body"], maxLogBody)
	assert.Equal(t, true, entry["res_body_truncated"])
	assert.Equal(t, float64(bodySize), entry["res_body_bytes"])

	// Assert: the handler and the client still get everything.
	assert.Equal(t, bodySize, handlerSaw, "handler must receive the full request body")
	assert.Equal(t, bodySize, w.Body.Len(), "client must receive the full response body")
}

func TestLogging_omits_body_for_webhooks(t *testing.T) {
	// Arrange.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/lever", strings.NewReader(`{"token":"secret"}`))

	// Act.
	out := captureStdout(t, func() {
		Logging(next).ServeHTTP(httptest.NewRecorder(), req)
	})

	// Assert.
	entry := parseLogEntry(t, out)
	_, hasReqBody := entry["req_body"]
	_, hasResBody := entry["res_body"]
	assert.False(t, hasReqBody, "request body must not be logged for webhook paths")
	assert.False(t, hasResBody, "response body must not be logged for webhook paths")
	assert.Equal(t, "/v1/webhooks/lever", entry["path"])
}

func TestLogging_omits_request_body_for_bodyless_methods(t *testing.T) {
	// Arrange.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)

	// Act.
	out := captureStdout(t, func() {
		Logging(next).ServeHTTP(httptest.NewRecorder(), req)
	})

	// Assert.
	entry := parseLogEntry(t, out)
	_, hasReqBody := entry["req_body"]
	assert.False(t, hasReqBody, "request body must not be logged for GET")
	assert.Equal(t, `{"ok":true}`, entry["res_body"])
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
