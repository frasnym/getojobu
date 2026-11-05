package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// Recovery catches panics and returns a 500 error envelope instead of crashing.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logging.FromContext(r.Context()).Error("http panic recovered", "panic", fmt.Sprint(rec))
				http.Error(w, `{"code":"internal","message":"internal server error"}`, http.StatusInternalServerError)
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
		ctx := logging.WithRequestID(r.Context(), reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logging logs each request with method, path, status, and latency.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		latency := time.Since(start)

		logger := logging.FromContext(r.Context())
		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"latency_ms", latency.Milliseconds(),
			"req_id", logging.RequestIDFromContext(r.Context()),
		)
	})
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
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
	return rec.ResponseWriter.Write(b)
}

// generateRequestID returns a short, unique request id.
func generateRequestID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())[:16]
}
