package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2/jwt"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// ValidClerkSvixSignature verifies Clerk webhook signatures using the Svix
// scheme. Requests with a missing or invalid signature are rejected with 401
// before reaching the handler. The raw body is restored so the downstream
// handler can decode it.
func (a *authMiddleware) ValidClerkSvixSignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			httpx.Envelope(httpx.CodeInvalidRequest, "failed to read body", "").WriteJSON(w)
			return
		}

		if !verifySvixSignature(a.cfg.Clerk.WebhookSecret, r.Header, body) {
			httpx.Envelope(httpx.CodeUnauthorized, "invalid webhook signature", "").WriteJSON(w)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// ValidClerkSession verifies a Clerk session JWT and resolves it to the
// internal application user.
func (a *authMiddleware) ValidClerkSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			httpx.Envelope(httpx.CodeUnauthorized, "missing authorization header", "").WriteJSON(w)
			return
		}

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

// isAuthorized checks whether the given authorized party is trusted.
// When no authorized parties are configured, all parties are accepted.
func (a *authMiddleware) isAuthorized(azp string) bool {
	if len(a.cfg.Clerk.AuthorizedParties) == 0 {
		return true
	}
	for _, p := range a.cfg.Clerk.AuthorizedParties {
		if p == azp {
			return true
		}
	}
	return false
}

// verifySvixSignature validates the Svix signature header against the raw body.
// svix-signature header format: v1,<base64_hmac>
// Message to sign: svix-id + "." + svix-timestamp + "." + raw_body
func verifySvixSignature(webhookSecret string, header http.Header, body []byte) bool {
	signature := header.Get("svix-signature")
	timestamp := header.Get("svix-timestamp")
	id := header.Get("svix-id")

	if signature == "" || timestamp == "" || id == "" {
		return false
	}

	parts := strings.SplitN(signature, ",", 2)
	if len(parts) != 2 || parts[0] != "v1" {
		return false
	}

	expectedMAC, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	message := id + "." + timestamp + "." + string(body)
	computedMAC := hmacSha256([]byte(webhookSecret), []byte(message))

	return hmac.Equal(expectedMAC, computedMAC)
}

func hmacSha256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
