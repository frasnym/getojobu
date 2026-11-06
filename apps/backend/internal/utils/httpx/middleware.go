package httpx

import "net/http"

// MiddlewareFunc wraps an http.Handler with middleware behavior.
// This follows the standard Go middleware pattern: func(http.Handler) http.Handler.
// Use it for per-route middleware:
//
//	rt.GET("/protected", h.Handler, authMiddleware)
type MiddlewareFunc func(http.Handler) http.Handler
