package middleware

import (
	"net/http"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
)

// skipPrefix checks if path starts with the given prefix.
func skipPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

// extractBearerToken extracts the bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// authMiddleware is an instance-based per-route auth middleware, mirroring
// the Echo-style pattern:
//
//	rt.GET("/protected", h.Handler, authMiddleware.ValidClerkSession)
//
// Add services to the struct as more auth methods are needed.
type authMiddleware struct {
	cfg    *config.Config
	userUC ucUsers.User
}

// NewAuthMiddleware creates an auth middleware instance from config and
// identity services.
func NewAuthMiddleware(cfg *config.Config, userUC ucUsers.User) AuthMiddleware {
	return &authMiddleware{
		cfg:    cfg,
		userUC: userUC,
	}
}

// AuthMiddleware is the interface for per-route auth middleware. Each method
// IS an httpx.MiddlewareFunc (func(http.Handler) http.Handler), so method
// values can be passed directly without parens:
//
//	rt.GET("/protected", h.Handler, auth.ValidClerkSession)
type AuthMiddleware interface {
	// ValidClerkSvixSignature verifies Clerk webhook signatures using the Svix scheme.
	ValidClerkSvixSignature(next http.Handler) http.Handler
	// ValidClerkSession verifies a Clerk session JWT and resolves it to the
	// internal application user. Status codes:
	//   - 401: missing or invalid token, untrusted authorized party, no local user
	//   - 403: local account deactivated
	ValidClerkSession(next http.Handler) http.Handler
	// ClerkOrAPIKey authenticates with either a machine API key (fast path,
	// server-side key-to-identity mapping) or a Clerk session JWT.
	ClerkOrAPIKey(next http.Handler) http.Handler
}
