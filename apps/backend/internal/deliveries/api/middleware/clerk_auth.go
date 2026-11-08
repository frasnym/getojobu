package middleware

import (
	"bytes"
	"context"
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
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// ValidClerkSvixSignature verifies Clerk webhook signatures using the Svix
// scheme. Requests with a missing or invalid signature are rejected with 401
// before reaching the handler. The raw body is restored so the downstream
// handler can decode it.
func (a *authMiddleware) ValidClerkSvixSignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logging.Error(r.Context(), "failed to read request body", err)
			httpx.Envelope(httpx.CodeInvalidRequest, "failed to read body", "").WriteJSON(w)
			return
		}

		if !verifySvixSignature(r.Context(), a.cfg.Clerk.WebhookSecret, r.Header, body) {
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
// The signing key is the Svix secret with the "whsec_" prefix stripped and the
// remainder base64-decoded (matching the official svix library behavior).
func verifySvixSignature(ctx context.Context, webhookSecret string, header http.Header, body []byte) bool {
	signature := header.Get("svix-signature")
	timestamp := header.Get("svix-timestamp")
	id := header.Get("svix-id")

	if signature == "" || timestamp == "" || id == "" {
		logging.Warn(ctx, "webhook signature missing headers",
			"hasSignature", signature != "",
			"hasTimestamp", timestamp != "",
			"hasID", id != "")
		return false
	}

	parts := strings.SplitN(signature, ",", 2)
	if len(parts) != 2 || parts[0] != "v1" {
		logging.Warn(ctx, "webhook signature invalid format",
			"prefix", parts[0],
			"partsCount", len(parts))
		return false
	}

	expectedMAC, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		logging.Error(ctx, "webhook signature base64 decode failed", err)
		return false
	}

	// Svix secrets are "whsec_<base64>"; the HMAC key is the base64-decoded
	// remainder, not the raw secret string.
	key, err := svixSigningKey(webhookSecret)
	if err != nil {
		logging.Error(ctx, "webhook secret decode failed", err)
		return false
	}

	message := id + "." + timestamp + "." + string(body)
	computedMAC := hmacSha256(key, []byte(message))

	isEqual := hmac.Equal(expectedMAC, computedMAC)
	if !isEqual {
		logging.Warn(ctx, "webhook signature mismatch",
			"expectedMAC", base64.StdEncoding.EncodeToString(expectedMAC),
			"computedMAC", base64.StdEncoding.EncodeToString(computedMAC))
	}
	return isEqual
}

// svixSigningKey derives the raw HMAC key from a Svix webhook secret.
// The secret is formatted as "whsec_<base64>"; the key is the base64-decoded
// remainder. A secret without the prefix is used as-is for flexibility.
func svixSigningKey(secret string) ([]byte, error) {
	encoded := strings.TrimPrefix(secret, "whsec_")
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func hmacSha256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
