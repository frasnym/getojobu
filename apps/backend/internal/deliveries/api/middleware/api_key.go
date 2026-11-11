package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// ValidClerkOrAPIKey authenticates a request with either a machine API key or a
// Clerk session JWT.
//
// Machine keys are checked first (fast path). On a match the request is
// attributed to the machine actor mapped in cfg.MachineAuth.APIKeys.
// Otherwise, it delegates directly to ValidClerkSession.
//
// Client headers such as X-User-Name are never read for identity.
func (a *authMiddleware) ValidClerkOrAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fast path: machine API key -> server-side key-to-identity mapping.
		if principal, ok := a.tryMachineAuth(r); ok {
			ctx := ctxval.ContextWithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Fallback: Clerk session JWT -> local user.
		a.ValidClerkSession(next).ServeHTTP(w, r)
	})
}

// ValidAPIKey authenticates a request strictly with a machine API key.
// Any missing authorization header, unknown key, or Clerk JWT is rejected
// with 401 Unauthorized ("missing or invalid api key").
func (a *authMiddleware) ValidAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			httpx.Envelope(httpx.CodeUnauthorized, "missing authorization header", "").WriteJSON(w)
			return
		}

		if principal, ok := a.tryMachineAuth(r); ok {
			ctx := ctxval.ContextWithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		httpx.Envelope(httpx.CodeUnauthorized, "missing or invalid api key", "").WriteJSON(w)
	})
}

// tryMachineAuth extracts the bearer token and attempts to match it against configured
// machine API keys. Returns the mapped Principal if matched.
func (a *authMiddleware) tryMachineAuth(r *http.Request) (*models.Principal, bool) {
	token := extractBearerToken(r)
	if token == "" {
		return nil, false
	}

	if key, ok := a.matchMachineKey(token); ok {
		return &models.Principal{
			ActorType: key.ActorType,
			ActorID:   key.ActorID,
		}, true
	}
	return nil, false
}

// matchMachineKey compares the bearer token against every configured machine
// API key using subtle.ConstantTimeCompare and returns the matching key (if
// any). All keys are compared even after a match so the comparison result does
// not depend on which key matched.
func (a *authMiddleware) matchMachineKey(token string) (config.MachineKey, bool) {
	var matched config.MachineKey
	found := false
	tokenBytes := []byte(token)
	for apiKey, key := range a.cfg.MachineAuth.APIKeys {
		if subtle.ConstantTimeCompare(tokenBytes, []byte(apiKey)) == 1 {
			matched = key
			found = true
		}
	}
	return matched, found
}
