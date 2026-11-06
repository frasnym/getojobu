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
	routes []Route
}

// New constructs a Router backed by a method-aware http.ServeMux.
func New() *Router {
	return &Router{
		mux: http.NewServeMux(),
	}
}

// Handle registers a handler for the given method and pattern.
// The method is prefixed to the pattern so ServeMux dispatches on both
// method and path (method-aware routing, Go 1.22+). Patterns support
// path wildcards ({name}) as supported by Go 1.22+ ServeMux.
func (r *Router) Handle(method, pattern string, h HandlerFunc) {
	methodPattern := method + " " + pattern
	r.mux.Handle(methodPattern, h)
	r.routes = append(r.routes, Route{
		Method:      method,
		Pattern:     pattern,
		HandlerName: nameOf(h),
	})
}

// GET registers a handler for GET method and pattern.
func (r *Router) GET(pattern string, h HandlerFunc) {
	r.Handle(http.MethodGet, pattern, h)
}

// POST registers a handler for POST method and pattern.
func (r *Router) POST(pattern string, h HandlerFunc) {
	r.Handle(http.MethodPost, pattern, h)
}

// PUT registers a handler for PUT method and pattern.
func (r *Router) PUT(pattern string, h HandlerFunc) {
	r.Handle(http.MethodPut, pattern, h)
}

// PATCH registers a handler for PATCH method and pattern.
func (r *Router) PATCH(pattern string, h HandlerFunc) {
	r.Handle(http.MethodPatch, pattern, h)
}

// DELETE registers a handler for DELETE method and pattern.
func (r *Router) DELETE(pattern string, h HandlerFunc) {
	r.Handle(http.MethodDelete, pattern, h)
}

// Mux returns the underlying ServeMux for server wiring.
func (r *Router) Mux() *http.ServeMux {
	return r.mux
}

// Routes returns all registered routes for introspection.
func (r *Router) Routes() []Route {
	return append([]Route(nil), r.routes...)
}

// EnvelopeHandler wraps h so that ServeMux's 404 and 405 responses are
// rewritten as the standard JSON error envelope instead of plain text.
func (r *Router) EnvelopeHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rec := &envelopeRecorder{ResponseWriter: w}
		h.ServeHTTP(rec, req)

		switch rec.status {
		case http.StatusNotFound:
			Envelope(CodeNotFound, "route not found", "").WriteJSON(w)
		case http.StatusMethodNotAllowed:
			Envelope(CodeMethodNotAllowed, "method not allowed", "").WriteJSON(w)
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
	rec.buffer = append(rec.buffer, b...)
	return len(b), nil
}

// nameOf returns a best-effort name for a handler, used for route introspection.
func nameOf(h HandlerFunc) string {
	return fmt.Sprintf("%T", h)
}
