package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// maxLogBody caps how much of a request or response body is written to the log.
// Anything beyond it is dropped and flagged as truncated.
const maxLogBody = 4096

// Recovery catches panics and returns a 500 error envelope instead of crashing.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logging.Error(r.Context(), "http panic recovered", fmt.Errorf("%v", rec))
				httpx.Envelope(httpx.CodeInternal, "internal server error", "").WriteJSON(w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestID extracts or generates a request id and stores it in the context.
// It echoes the X-Request-Id request header when present, otherwise generates one.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = r.Header.Get("X-Correlation-Id")
		}
		if reqID == "" {
			reqID = generateRequestID()
		}
		w.Header().Set("X-Request-Id", reqID)
		ctx := ctxval.WithRequestID(r.Context(), reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logging logs each request with method, path, status, latency, and a bounded
// copy of the request and response bodies.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		reqBodyAllowed := logRequestBodyAllowed(r)
		resBodyAllowed := !skipBodyLog(r)
		reqBody, reqBytes, reqTruncated := captureRequestBody(r)

		next.ServeHTTP(rec, r)
		latency := time.Since(start)

		args := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"latency_ms", latency.Milliseconds(),
		}
		if reqBodyAllowed && reqBytes > 0 {
			args = append(args,
				"req_body", string(reqBody),
				"req_body_bytes", reqBytes,
				"req_body_truncated", reqTruncated,
			)
		}
		if resBodyAllowed && rec.size > 0 {
			args = append(args,
				"res_body", string(rec.body),
				"res_body_bytes", rec.size,
				"res_body_truncated", rec.truncated,
			)
		}

		logging.Info(r.Context(), "http request", args...)
	})
}

// skipBodyLog reports whether bodies must never be logged for this path.
// Webhook payloads and responses carry third-party secrets.
func skipBodyLog(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/v1/webhooks")
}

// logRequestBodyAllowed reports whether the request body is safe and useful to
// log. Bodyless methods have no request body to show.
func logRequestBodyAllowed(r *http.Request) bool {
	if skipBodyLog(r) {
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodDelete:
		return false
	}
	return true
}

// captureRequestBody reads at most maxLogBody bytes of the request body and puts
// the body back so the handler still sees the full payload. It returns the
// captured bytes, the full request size when known, and whether it was cut short.
func captureRequestBody(r *http.Request) (body []byte, size int, truncated bool) {
	if !logRequestBodyAllowed(r) || r.Body == nil || r.Body == http.NoBody {
		return nil, 0, false
	}

	buf := make([]byte, maxLogBody+1)
	n, err := io.ReadFull(r.Body, buf)
	switch {
	case err == nil:
		truncated = true
	case err == io.EOF, err == io.ErrUnexpectedEOF:
		// Body fits in the buffer.
	default:
		// Read error: keep what was read, leave the rest for the handler.
	}

	captured := buf[:n]
	if truncated {
		captured = buf[:maxLogBody]
	}

	rest := r.Body
	r.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(buf[:n]), rest), Closer: rest}

	size = len(buf[:n])
	if r.ContentLength > 0 {
		size = int(r.ContentLength)
	}
	return captured, size, truncated
}

// readCloser joins a replay reader with the original body's Close.
type readCloser struct {
	io.Reader
	io.Closer
}

// statusRecorder wraps http.ResponseWriter to capture the status code and a
// bounded copy of the response body.
type statusRecorder struct {
	http.ResponseWriter
	status    int
	wrote     bool
	body      []byte
	size      int
	truncated bool
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.wrote = true
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	if !rec.wrote {
		rec.WriteHeader(http.StatusOK)
	}
	rec.size += len(b)
	if remaining := maxLogBody - len(rec.body); remaining > 0 {
		if len(b) <= remaining {
			rec.body = append(rec.body, b...)
		} else {
			rec.body = append(rec.body, b[:remaining]...)
			rec.truncated = true
		}
	} else if len(b) > 0 {
		rec.truncated = true
	}
	return rec.ResponseWriter.Write(b)
}

// generateRequestID returns a short, unique request id.
func generateRequestID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())[:16]
}
