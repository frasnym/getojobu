package user_jobs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucUserJobsMock "github.com/frasnym/getojobu/apps/backend/internal/usecase/user_jobs/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newHandlerWithMock(ctrl *gomock.Controller) (*handler, *ucUserJobsMock.MockUserJobs) {
	mockJobs := ucUserJobsMock.NewMockUserJobs(ctrl)
	h := &handler{
		cfg:    &config.Config{},
		ucJobs: mockJobs,
	}
	return h, mockJobs
}

func principalCtx(id string) *models.Principal {
	return &models.Principal{ID: id, Role: models.RoleUser}
}

func bodyAsJobs(t *testing.T, w *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp httpx.Response[[]map[string]interface{}]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	return resp.Data
}

func TestListUserJobs_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		ListByUser(gomock.Any(), "user_1", 20).
		Return([]*models.Job{{ID: 1, Slug: "job-1", Title: "Engineer"}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	jobs := bodyAsJobs(t, w)
	require.Len(t, jobs, 1)
	assert.Equal(t, float64(1), jobs[0]["id"])
	assert.Equal(t, "job-1", jobs[0]["slug"])
	assert.Equal(t, "Engineer", jobs[0]["title"])
}

func TestListUserJobs_empty(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		ListByUser(gomock.Any(), "user_1", 20).
		Return([]*models.Job{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	jobs := bodyAsJobs(t, w)
	require.Len(t, jobs, 0)
}

func TestListUserJobs_unauthenticated(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs", nil)
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListUserJobs_limit_clamped(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		ListByUser(gomock.Any(), "user_1", 100).
		Return([]*models.Job{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs?limit=999", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListUserJobs_invalid_limit(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs?limit=abc", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRoute_registers_userjobs_under_group(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)
	rt := httpx.New()
	group := rt.Group("/v1/user-jobs")

	// Act.
	h.Route(group)

	// Assert via introspection. Patterns are compounded with the group prefix.
	routes := rt.Routes()
	require.Len(t, routes, 1, "expected 1 user-jobs route under /v1/user-jobs")
	assert.Equal(t, http.MethodGet, routes[0].Method)
	assert.Equal(t, "/v1/user-jobs", routes[0].Pattern)
}
