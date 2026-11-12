package handler

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/experiences"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/jobs"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/projects"
	scoringConfigs "github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/scoringconfigs"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/webhook"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	httpSwagger "github.com/swaggo/http-swagger"
)

// handler holds shared dependencies for every HTTP handler. Route methods
// have the signature func(http.ResponseWriter, *http.Request) error, so a
// handler can return an error instead of writing a response inline.
type handler struct {
	contract *contract.Contract
}

// New wires the handler set from the contract. contract.New guarantees Cfg,
// Common, Repository, and Usecase are non-nil, so they are used directly.
func New(c *contract.Contract) *handler {
	return &handler{contract: c}
}

// Route registers every HTTP route for this handler set.
// Clerk auth is applied per-route via authMiddleware (Echo-style),
// not as a global middleware chain.
// Usage: rt.GET("/protected", h.ProtectedHandler, auth.ValidClerkSession)
func (h *handler) Route(rt *httpx.Router) {
	// All API routes are grouped under /v1/.
	v1 := rt.Group("/v1")

	// Health and Swagger are at the root (not under /v1).
	rt.GET("/health", h.Health)

	// Swagger UI at /swagger/ (serves docs/swagger.json spec).
	// Gated behind a bearer token when SWAGGER_TOKEN is set (production).
	// Local dev (token unset) stays open.
	rt.GET("/swagger/", httpx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		httpSwagger.Handler().ServeHTTP(w, r)
		return nil
	}), h.swaggerAuth)

	// Clerk webhook under /v1/webhooks group.
	webhook.NewHandler(h.contract).Route(v1.Group("/webhooks"))

	// Experience routes (CRU only; no DELETE).
	experiences.NewHandler(h.contract).Route(v1.Group("/experiences"))

	// Project routes (CRU only; no DELETE).
	projects.NewHandler(h.contract).Route(v1.Group("/projects"))

	// Job routes (CRUD + bulk create + scrape orchestration).
	jobs.NewHandler(h.contract).Route(v1.Group("/jobs"))

	// Scoring configuration routes (CRU only; machine API key only).
	scoringConfigs.NewHandler(h.contract).Route(v1.Group("/scoring-configs"))
}

// swaggerAuth gates the Swagger UI behind HTTP Basic Auth when SWAGGER_TOKEN
// is set (production). A 401 with WWW-Authenticate: Basic triggers the
// browser's native username/password popup. Any username works; the password
// must equal SWAGGER_TOKEN. Local dev (token unset) stays open.
func (h *handler) swaggerAuth(next http.Handler) http.Handler {
	token := h.contract.Cfg.Server.SwaggerToken
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Local dev: token not configured, keep Swagger open.
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Production: require HTTP Basic Auth. The browser shows a popup
		// asking for username/password; any username works, password must
		// equal SWAGGER_TOKEN.
		const basicPrefix = "Basic "
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, basicPrefix) {
			challenge(w)
			return
		}

		encoded := auth[len(basicPrefix):]
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			challenge(w)
			return
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 || parts[1] != token {
			challenge(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// challenge sends a 401 with a Basic auth challenge, which makes the
// browser show its native login popup.
func challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="swagger"`)
	w.WriteHeader(http.StatusUnauthorized)
}
