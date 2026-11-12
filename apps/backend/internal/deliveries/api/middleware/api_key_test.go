package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

type stubUserUC struct{}

func (stubUserUC) FindByClerkUserID(context.Context, string) (*models.Principal, error) {
	return nil, nil
}
func (stubUserUC) FindByID(context.Context, string) (*models.User, error) {
	return nil, nil
}
func (stubUserUC) Create(context.Context, string, string, string, models.Role) (*models.User, error) {
	return nil, nil
}
func (stubUserUC) Update(context.Context, string, string, string) (*models.User, error) {
	return nil, nil
}
func (stubUserUC) SoftDelete(context.Context, string) (*models.User, error) {
	return nil, nil
}

func TestValidClerkOrAPIKey_missing_authorization_header(t *testing.T) {
	m := &authMiddleware{cfg: &config.Config{}, userUC: stubUserUC{}}

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	m.ValidClerkOrAPIKey(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, called)
	var resp httpx.Response[any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Nil(t, resp.Data)
	require.NotNil(t, resp.Error)
	assert.Equal(t, httpx.CodeUnauthorized, resp.Error.Code)
	assert.Equal(t, "missing authorization header", resp.Error.Message)
}

func TestValidClerkOrAPIKey_invalid_bearer_scheme(t *testing.T) {
	m := &authMiddleware{cfg: &config.Config{}, userUC: stubUserUC{}}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	req.Header.Set("Authorization", "Basic whatever")
	w := httptest.NewRecorder()
	m.ValidClerkOrAPIKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidClerkOrAPIKey_machine_key_maps_to_principal(t *testing.T) {
	cfg := &config.Config{
		MachineAuth: config.MachineAuth{
			APIKeys: map[string]config.MachineKey{
				"test-cron-key":       {ActorType: "cron", ActorID: "cron-scrape"},
				"test-hermes-key":     {ActorType: "agent", ActorID: "hermes"},
				"test-playwright-key": {ActorType: "worker", ActorID: "playwright-worker"},
			},
		},
	}
	m := &authMiddleware{cfg: cfg, userUC: stubUserUC{}}

	var called bool
	var gotType, gotID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		p := ctxval.PrincipalFromContext(r.Context())
		assert.NotNil(t, p)
		gotType, gotID = p.AuditActor()
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	req.Header.Set("Authorization", "Bearer test-hermes-key")
	w := httptest.NewRecorder()
	m.ValidClerkOrAPIKey(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)
	assert.Equal(t, "agent", gotType)
	assert.Equal(t, "hermes", gotID)
}

func TestValidClerkOrAPIKey_unknown_key_falls_through_to_clerk_and_rejects(t *testing.T) {
	m := &authMiddleware{cfg: &config.Config{}, userUC: stubUserUC{}}

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	req.Header.Set("Authorization", "Bearer definitely-not-a-valid-key-or-jwt")
	w := httptest.NewRecorder()
	m.ValidClerkOrAPIKey(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, called)
}

func TestAuditActor_machine_actor(t *testing.T) {
	p := &models.Principal{ActorType: "cron", ActorID: "cron-scrape"}
	actorType, actorID := p.AuditActor()
	assert.Equal(t, "cron", actorType)
	assert.Equal(t, "cron-scrape", actorID)
}

func TestAuditActor_clerk_user(t *testing.T) {
	p := &models.Principal{ID: "u_123", ClerkUserID: "user_abc"}
	actorType, actorID := p.AuditActor()
	assert.Equal(t, "user", actorType)
	assert.Equal(t, "u_123", actorID)
}

func TestValidAPIKey_valid_machine_key(t *testing.T) {
	cfg := &config.Config{
		MachineAuth: config.MachineAuth{
			APIKeys: map[string]config.MachineKey{
				"worker-key": {ActorType: "worker", ActorID: "python-worker"},
			},
		},
	}
	m := &authMiddleware{cfg: cfg, userUC: stubUserUC{}}

	var called bool
	var gotType, gotID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		p := ctxval.PrincipalFromContext(r.Context())
		assert.NotNil(t, p)
		gotType, gotID = p.AuditActor()
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs", nil)
	req.Header.Set("Authorization", "Bearer worker-key")
	w := httptest.NewRecorder()
	m.ValidAPIKey(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)
	assert.Equal(t, "worker", gotType)
	assert.Equal(t, "python-worker", gotID)
}

func TestValidAPIKey_missing_header(t *testing.T) {
	m := &authMiddleware{cfg: &config.Config{}, userUC: stubUserUC{}}

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs", nil)
	w := httptest.NewRecorder()
	m.ValidAPIKey(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, called)
	var resp httpx.Response[any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "missing authorization header", resp.Error.Message)
}

func TestValidAPIKey_rejects_non_machine_key(t *testing.T) {
	cfg := &config.Config{
		MachineAuth: config.MachineAuth{
			APIKeys: map[string]config.MachineKey{
				"valid-key": {ActorType: "worker", ActorID: "worker-1"},
			},
		},
	}
	m := &authMiddleware{cfg: cfg, userUC: stubUserUC{}}

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs", nil)
	req.Header.Set("Authorization", "Bearer some-clerk-user-jwt-or-invalid-key")
	w := httptest.NewRecorder()
	m.ValidAPIKey(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, called)
	var resp httpx.Response[any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "missing or invalid api key", resp.Error.Message)
}
