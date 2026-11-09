package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2/jwt"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// ClerkOrAPIKey authenticates a request with either a machine API key or a
// Clerk session JWT.
//
// Machine keys are checked first (fast path). On a match the request is
// attributed to the machine actor mapped in cfg.MachineAuth.APIKeys
// (actor_type, actor_id) and no Clerk verification is performed. Otherwise the
// token is verified as a Clerk session JWT and resolved to a local user.
//
// Client headers such as X-User-Name are never read for identity.
func (a *authMiddleware) ClerkOrAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			httpx.Envelope(httpx.CodeUnauthorized, "missing authorization header", "").WriteJSON(w)
			return
		}

		// Fast path: machine API key -> server-side key-to-identity mapping.
		// Every configured key is compared with a constant-time comparison so
		// the result does not leak which key (if any) matches.
		if key, ok := a.matchMachineKey(token); ok {
			principal := &models.Principal{
				ActorType: key.ActorType,
				ActorID:   key.ActorID,
			}
			ctx := ctxval.ContextWithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Fallback: Clerk session JWT -> local user.
		claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
			Token: token,
		})
		if err != nil {
			httpx.Envelope(httpx.CodeUnauthorized, "invalid token", "").WriteJSON(w)
			return
		}
		if claims.Subject == "" {
			httpx.Envelope(httpx.CodeUnauthorized, "clerk token has no subject claim", "").WriteJSON(w)
			return
		}

		if !a.isAuthorized(claims.AuthorizedParty) {
			httpx.Envelope(httpx.CodeUnauthorized, "untrusted authorized party", "").WriteJSON(w)
			return
		}

		principal, err := a.userUC.FindByClerkUserID(r.Context(), claims.Subject)
		if err != nil {
			switch {
			case errors.Is(err, constants.ErrAccountDeactivated):
				httpx.Envelope(httpx.CodeForbidden, "account deactivated", "").WriteJSON(w)
				return
			case errors.Is(err, constants.ErrUserNotFound):
				httpx.Envelope(httpx.CodeUnauthorized, "user not found", "").WriteJSON(w)
				return
			default:
				httpx.Envelope(httpx.CodeInternal, "identity resolution failed", "").WriteJSON(w)
				return
			}
		}

		ctx := ctxval.ContextWithPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
