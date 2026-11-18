package me

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/usecase"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucUsersMock "github.com/frasnym/getojobu/apps/backend/internal/usecase/users/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

func newHandlerWithMock(ctrl *gomock.Controller) (*handler, *ucUsersMock.MockUser) {
	mockUser := ucUsersMock.NewMockUser(ctrl)
	h := &handler{
		cfg:    &config.Config{},
		userUC: mockUser,
	}
	return h, mockUser
}

func principalCtx(id string) *models.Principal {
	return &models.Principal{ID: id, Role: models.RoleUser}
}

func strPtr(s string) *string { return &s }

func sampleUser(id string) *models.User {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	return &models.User{
		ID:        id,
		Email:     strPtr("jane@example.com"),
		FullName:  strPtr("Jane Doe"),
		Role:      models.RoleUser,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func responseBody(t *testing.T, w *httptest.ResponseRecorder) httpx.Response[map[string]interface{}] {
	t.Helper()
	var resp httpx.Response[map[string]interface{}]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func hasDetailField(details []httpx.ErrorDetail, field string) bool {
	for _, d := range details {
		if d.Field == field {
			return true
		}
	}
	return false
}

func updateBodyJSON(vals map[string]interface{}) string {
	b, err := json.Marshal(vals)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func allSixNull() map[string]interface{} {
	return map[string]interface{}{
		"full_name":     nil,
		"headline":      nil,
		"phone":         nil,
		"portfolio_url": nil,
		"linkedin_url":  nil,
		"github_url":    nil,
	}
}

func TestGetMe_uses_principal_id(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockUser := newHandlerWithMock(ctrl)

	mockUser.EXPECT().
		FindByID(gomock.Any(), "usr-1").
		Return(sampleUser("usr-1"), nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/me?user_id=usr-2", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("usr-1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.GetMe).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	data := responseBody(t, w).Data
	_, hasClerkID := data["clerk_user_id"]
	assert.False(t, hasClerkID, "clerk_user_id must not be exposed")
	assert.Equal(t, "usr-1", data["id"])
}

func TestUpdateMe_rejects_role_email_and_unknown_fields(t *testing.T) {
	bodies := []string{
		`{"full_name":"Jane","role":"admin"}`,
		`{"full_name":"Jane","email":"x@y.z"}`,
		`{"full_name":"Jane","surprise":1}`,
	}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			h, _ := newHandlerWithMock(ctrl)

			req := httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("usr-1")))
			w := httptest.NewRecorder()

			httpx.HandlerFunc(h.UpdateMe).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			require.NotNil(t, responseBody(t, w).Error)
			assert.Equal(t, httpx.CodeInvalidRequest, responseBody(t, w).Error.Code)
		})
	}
}

func TestUpdateMe_rejects_unknown_field_with_all_keys_present(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	vals := allSixNull()
	vals["full_name"] = "Jane"
	vals["surprise"] = 1

	req := httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(updateBodyJSON(vals)))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("usr-1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.UpdateMe).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	require.NotNil(t, responseBody(t, w).Error)
	assert.Equal(t, httpx.CodeInvalidRequest, responseBody(t, w).Error.Code)
}

func TestUpdateMe_ignores_client_supplied_user_id(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockUser := newHandlerWithMock(ctrl)

	expected := models.UserProfileInput{
		FullName:     strPtr("Jane"),
		Headline:     strPtr("Backend Engineer"),
		Phone:        strPtr("+62 812"),
		PortfolioURL: strPtr("https://jane.dev"),
		LinkedInURL:  strPtr("https://linkedin.com/in/jane"),
		GitHubURL:    strPtr("https://github.com/jane"),
	}
	mockUser.EXPECT().
		UpdateProfile(gomock.Any(), "usr-1", expected).
		Return(sampleUser("usr-1"), nil)

	body := `{"full_name":"Jane","headline":"Backend Engineer","phone":"+62 812","portfolio_url":"https://jane.dev","linkedin_url":"https://linkedin.com/in/jane","github_url":"https://github.com/jane"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/me?user_id=usr-2", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("usr-1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.UpdateMe).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateMe_validates_fields(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		jsonKey string
	}{
		{"bad portfolio url", "portfolio_url", "not-a-url", "portfolio_url"},
		{"bad linkedin scheme", "linkedin_url", "ftp://example.com", "linkedin_url"},
		{"blank full name", "full_name", "   ", "full_name"},
		{"long headline", "headline", strings.Repeat("x", 257), "headline"},
		{"long phone", "phone", strings.Repeat("x", 65), "phone"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			h, _ := newHandlerWithMock(ctrl)

			vals := allSixNull()
			vals[tt.field] = tt.value

			req := httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(updateBodyJSON(vals)))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("usr-1")))
			w := httptest.NewRecorder()

			httpx.HandlerFunc(h.UpdateMe).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			resp := responseBody(t, w)
			require.NotNil(t, resp.Error)
			assert.Equal(t, httpx.CodeInvalidRequest, resp.Error.Code)
			assert.True(t, hasDetailField(resp.Error.Details, tt.jsonKey),
				"expected a validation detail for %q, got %+v", tt.jsonKey, resp.Error.Details)
		})
	}
}

func TestUpdateMe_all_six_fields_required(t *testing.T) {
	fields := []string{"full_name", "headline", "phone", "portfolio_url", "linkedin_url", "github_url"}
	for _, missing := range fields {
		t.Run("missing_"+missing, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			h, _ := newHandlerWithMock(ctrl)

			vals := allSixNull()
			delete(vals, missing)

			req := httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(updateBodyJSON(vals)))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("usr-1")))
			w := httptest.NewRecorder()

			httpx.HandlerFunc(h.UpdateMe).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			resp := responseBody(t, w)
			require.NotNil(t, resp.Error)
			assert.Equal(t, httpx.CodeInvalidRequest, resp.Error.Code)
			assert.True(t, hasDetailField(resp.Error.Details, missing),
				"expected a validation detail for missing %q, got %+v", missing, resp.Error.Details)
		})
	}
}

func TestDeactivateMe_soft_deletes_principal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockUser := newHandlerWithMock(ctrl)

	mockUser.EXPECT().
		SoftDelete(gomock.Any(), "usr-1").
		Return(sampleUser("usr-1"), nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/me/deactivate", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("usr-1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.DeactivateMe).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoute_registers_me_routes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockUser := ucUsersMock.NewMockUser(ctrl)
	c := &contract.Contract{
		Cfg:     &config.Config{},
		Usecase: &usecase.Contract{Users: mockUser},
	}

	rt := httpx.New()
	NewHandler(c).Route(rt.Group("/v1/me"))

	routes := rt.Routes()
	require.Len(t, routes, 3)
	assert.Equal(t, http.MethodGet, routes[0].Method)
	assert.Equal(t, "/v1/me", routes[0].Pattern)
	assert.Equal(t, http.MethodPatch, routes[1].Method)
	assert.Equal(t, "/v1/me", routes[1].Pattern)
	assert.Equal(t, http.MethodPost, routes[2].Method)
	assert.Equal(t, "/v1/me/deactivate", routes[2].Pattern)
}

func TestGetMe_unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.GetMe).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
