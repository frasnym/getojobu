package router

import (
	"fmt"
	"net/http"

	httperr "github.com/frasnym/getojobu/apps/backend/internal/utils/http"
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
// path wildcards (*) as supported by Go 1.22+ ServeMux.
func (r *Router) Handle(method, pattern string, h http.HandlerFunc) {
	methodPattern := method + " " + pattern
	r.mux.Handle(methodPattern, http.HandlerFunc(h))
	r.routes = append(r.routes, Route{
		Method:      method,
		Pattern:     pattern,
		HandlerName: runtimeFuncName(h),
	})
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
			httperr.Envelope(httperr.CodeNotFound, "route not found", "").WriteJSON(w)
		case http.StatusMethodNotAllowed:
			httperr.Envelope(httperr.CodeMethodNotAllowed, "method not allowed", "").WriteJSON(w)
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

// runtimeFuncName returns a best-effort name for a handler function.
// This is intentionally simple; it is used only for route introspection.
func runtimeFuncName(h http.HandlerFunc) string {
	return fmt.Sprintf("%T", h)
}

// PathValue extracts the named path parameter from the request,
// mirroring http.Request.PathValue for patterns with {name} placeholders.
func PathValue(req *http.Request, name string) string {
	return req.PathValue(name)
}
