package experiences

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newHandlerWithMock(ctrl *gomock.Controller) (*handler, *mocks.MockExperiences) {
	mockExp := mocks.NewMockExperiences(ctrl)
	h := &handler{
		cfg:   &config.Config{},
		ucExp: mockExp,
	}
	return h, mockExp
}

func principalCtx(id string) *models.Principal {
	return &models.Principal{ID: id, Role: models.RoleUser}
}

func bodyAsMap(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp httpx.Response[map[string]interface{}]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	return resp.Data
}

func TestCreateExperience_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockExp := newHandlerWithMock(ctrl)

	mockExp.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(&models.Experience{ID: "exp_1", UserID: "user_1"}, nil)

	body := strings.NewReader(`{"experience_type":"professional","organization":"Acme","role_title":"Engineer","is_current":false}`)
	req := httptest.NewRequest(http.MethodPost, "/experiences", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateExperience).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusCreated, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "exp_1", m["ID"])
	assert.Equal(t, "user_1", m["UserID"])
}

func TestCreateExperience_unauthenticated(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"experience_type":"professional"}`)
	req := httptest.NewRequest(http.MethodPost, "/experiences", body)
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateExperience).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateExperience_malformed_body(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPost, "/experiences", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateExperience).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateExperience_invalid_experience_type(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"experience_type":"bogus","organization":"Acme"}`)
	req := httptest.NewRequest(http.MethodPost, "/experiences", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateExperience).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateExperience_missing_experience_type(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"organization":"Acme"}`)
	req := httptest.NewRequest(http.MethodPost, "/experiences", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateExperience).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetExperience_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockExp := newHandlerWithMock(ctrl)

	mockExp.EXPECT().
		GetByID(gomock.Any(), models.GetExperienceParams{UserID: "user_1", ID: "exp_1"}).
		Return(&models.Experience{ID: "exp_1", UserID: "user_1"}, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /experiences/{id}", httpx.HandlerFunc(h.GetExperience))

	req := httptest.NewRequest(http.MethodGet, "/experiences/exp_1", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "exp_1", m["ID"])
	assert.Equal(t, "user_1", m["UserID"])
}

func TestGetExperience_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockExp := newHandlerWithMock(ctrl)

	mockExp.EXPECT().
		GetByID(gomock.Any(), models.GetExperienceParams{UserID: "user_1", ID: "exp_1"}).
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("GET /experiences/{id}", httpx.HandlerFunc(h.GetExperience))

	req := httptest.NewRequest(http.MethodGet, "/experiences/exp_1", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetExperience_missing_id(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("GET /experiences/{id}", httpx.HandlerFunc(h.GetExperience))

	req := httptest.NewRequest(http.MethodGet, "/experiences/", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert. "/experiences/" does not match "/experiences/{id}" so ServeMux returns 404.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListExperiences_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockExp := newHandlerWithMock(ctrl)

	mockExp.EXPECT().
		ListByUser(gomock.Any(), "user_1").
		Return([]*models.Experience{{ID: "exp_1", UserID: "user_1"}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/experiences", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListExperiences).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]map[string]interface{}]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "exp_1", resp.Data[0]["ID"])
	assert.Equal(t, "user_1", resp.Data[0]["UserID"])
}

func TestUpdateExperience_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockExp := newHandlerWithMock(ctrl)

	mockExp.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(&models.Experience{ID: "exp_1", UserID: "user_1"}, nil)

	mux := http.NewServeMux()
	mux.Handle("PUT /experiences/{id}", httpx.HandlerFunc(h.UpdateExperience))

	body := strings.NewReader(`{"experience_type":"freelance","organization":"LLC","role_title":"Consultant","is_current":false}`)
	req := httptest.NewRequest(http.MethodPut, "/experiences/exp_1", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "exp_1", m["ID"])
	assert.Equal(t, "user_1", m["UserID"])
}

func TestUpdateExperience_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockExp := newHandlerWithMock(ctrl)

	mockExp.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("PUT /experiences/{id}", httpx.HandlerFunc(h.UpdateExperience))

	body := strings.NewReader(`{"experience_type":"professional","organization":"Acme"}`)
	req := httptest.NewRequest(http.MethodPut, "/experiences/exp_1", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateExperience_missing_id(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PUT /experiences/{id}", httpx.HandlerFunc(h.UpdateExperience))

	body := strings.NewReader(`{"experience_type":"professional"}`)
	req := httptest.NewRequest(http.MethodPut, "/experiences/", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert. "/experiences/" does not match "/experiences/{id}" so ServeMux returns 404.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateExperience_invalid_experience_type(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PUT /experiences/{id}", httpx.HandlerFunc(h.UpdateExperience))

	body := strings.NewReader(`{"experience_type":"bogus","organization":"Acme"}`)
	req := httptest.NewRequest(http.MethodPut, "/experiences/exp_1", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateExperience_malformed_body(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PUT /experiences/{id}", httpx.HandlerFunc(h.UpdateExperience))

	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPut, "/experiences/exp_1", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRoute_registers_experiences_under_group(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)
	rt := httpx.New()
	group := rt.Group("/v1/experiences")

	// Act.
	h.Route(group)

	// Assert via introspection. Patterns are compounded with the group prefix.
	routes := rt.Routes()
	require.Len(t, routes, 4, "expected 4 experience routes under /v1/experiences")
	expected := []struct {
		method  string
		pattern string
	}{
		{http.MethodPost, "/v1/experiences"},
		{http.MethodGet, "/v1/experiences"},
		{http.MethodGet, "/v1/experiences/{id}"},
		{http.MethodPut, "/v1/experiences/{id}"},
	}
	for i, e := range expected {
		assert.Equal(t, e.method, routes[i].Method, "route %d method", i)
		assert.Equal(t, e.pattern, routes[i].Pattern, "route %d pattern", i)
	}
}
