package httpx

import (
	"fmt"
	"net/http"
)

// Route describes a registered route for introspection.
type Route struct {
	Method      string
	Pattern     string
	HandlerName string
}

// Router wraps http.ServeMux to surface method-aware routing (404 vs 405)
// and route introspection.
type Router struct {
	mux    *http.ServeMux
	routes *[]Route
	prefix string
}

// New constructs a Router backed by a method-aware http.ServeMux.
func New() *Router {
	routes := []Route{}
	return &Router{
		mux:    http.NewServeMux(),
		routes: &routes,
	}
}

// Group returns a new Router that shares the underlying mux and route
// registry. Every pattern registered through the group is prefixed with
// prefix, so related routes can be grouped without repeating the prefix:
//
//	v1 := rt.Group("/v1")
//	v1.GET("/health", h.Health)
//	v1.POST("/webhooks/clerk", h.Webhook, authMiddleware)
func (r *Router) Group(prefix string) *Router {
	return &Router{
		mux:    r.mux,
		routes: r.routes,
		prefix: r.prefix + prefix,
	}
}

// Handle registers a handler for the given method and pattern.
// The method is prefixed to the pattern so ServeMux dispatches on both
// method and path (method-aware routing, Go 1.22+). Patterns support
// path wildcards ({name}) as supported by Go 1.22+ ServeMux.
// Middleware functions are applied in order: the first middleware wraps
// the handler, the second wraps the result, etc.
// Usage: rt.GET("/protected", h.HandlerFunc, authMiddleware)
func (r *Router) Handle(method, pattern string, h HandlerFunc, m ...MiddlewareFunc) {
	// Apply middleware in reverse so the first listed middleware is outermost.
	handler := http.Handler(h)
	for i := len(m) - 1; i >= 0; i-- {
		handler = m[i](handler)
	}

	methodPattern := method + " " + r.prefix + pattern
	r.mux.Handle(methodPattern, handler)
	*r.routes = append(*r.routes, Route{
		Method:      method,
		Pattern:     r.prefix + pattern,
		HandlerName: nameOf(h),
	})
}

// GET registers a handler for GET method and pattern.
// Middleware functions are applied in order (Echo-style per-route middleware).
func (r *Router) GET(pattern string, h HandlerFunc, m ...MiddlewareFunc) {
	r.Handle(http.MethodGet, pattern, h, m...)
}

// POST registers a handler for POST method and pattern.
// Middleware functions are applied in order (Echo-style per-route middleware).
func (r *Router) POST(pattern string, h HandlerFunc, m ...MiddlewareFunc) {
	r.Handle(http.MethodPost, pattern, h, m...)
}

// PUT registers a handler for PUT method and pattern.
// Middleware functions are applied in order (Echo-style per-route middleware).
func (r *Router) PUT(pattern string, h HandlerFunc, m ...MiddlewareFunc) {
	r.Handle(http.MethodPut, pattern, h, m...)
}

// PATCH registers a handler for PATCH method and pattern.
// Middleware functions are applied in order (Echo-style per-route middleware).
func (r *Router) PATCH(pattern string, h HandlerFunc, m ...MiddlewareFunc) {
	r.Handle(http.MethodPatch, pattern, h, m...)
}

// DELETE registers a handler for DELETE method and pattern.
// Middleware functions are applied in order (Echo-style per-route middleware).
func (r *Router) DELETE(pattern string, h HandlerFunc, m ...MiddlewareFunc) {
	r.Handle(http.MethodDelete, pattern, h, m...)
}

// Mux returns the underlying ServeMux for server wiring.
func (r *Router) Mux() *http.ServeMux {
	return r.mux
}

// Routes returns all registered routes for introspection.
func (r *Router) Routes() []Route {
	return append([]Route(nil), *r.routes...)
}

// EnvelopeHandler wraps h so that ServeMux's 404 and 405 responses are
// rewritten as the standard JSON error envelope instead of plain text.
func (r *Router) EnvelopeHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rec := &envelopeRecorder{ResponseWriter: w}
		h.ServeHTTP(rec, req)

		switch rec.status {
		case http.StatusNotFound:
			Envelope(CodeNotFound, "route not found", "").writeJSON(w, req)
		case http.StatusMethodNotAllowed:
			Envelope(CodeMethodNotAllowed, "method not allowed", "").writeJSON(w, req)
		default:
			rec.ResponseWriter.WriteHeader(rec.status)
			if len(rec.buffer) > 0 {
				rec.ResponseWriter.Write(rec.buffer)
			}
		}
	})
}

// envelopeRecorder buffers the response so 404/405 can be rewritten as JSON.
type envelopeRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
	buffer []byte
}

func (rec *envelopeRecorder) WriteHeader(code int) {
	rec.status = code
	rec.wrote = true
	// Defer writing headers until EnvelopeHandler decides whether to rewrite.
}

func (rec *envelopeRecorder) Write(b []byte) (int, error) {
	// Auto-call WriteHeader(200) if the handler wrote without setting a status,
	// so the underlying ResponseWriter always gets a valid status code.
	if !rec.wrote {
		rec.WriteHeader(http.StatusOK)
	}
	rec.buffer = append(rec.buffer, b...)
	return len(b), nil
}

// nameOf returns a best-effort name for a handler, used for route introspection.
func nameOf(h HandlerFunc) string {
	return fmt.Sprintf("%T", h)
}
