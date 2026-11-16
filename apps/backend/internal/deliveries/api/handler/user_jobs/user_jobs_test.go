package user_jobs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucUserJobsMock "github.com/frasnym/getojobu/apps/backend/internal/usecase/user_jobs/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
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

func bodyAsAddUserJob(t *testing.T, w *httptest.ResponseRecorder) addUserJobResponse {
	t.Helper()
	var resp httpx.Response[addUserJobResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	return resp.Data
}

func bodyAsSetHideStatus(t *testing.T, w *httptest.ResponseRecorder) setHideStatusResponse {
	t.Helper()
	var resp httpx.Response[setHideStatusResponse]
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
		ListByUser(gomock.Any(), "user_1", 10, 0).
		Return([]*models.Job{{ID: 1, Slug: "job-1", Title: "Engineer"}}, 1, nil)

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
		ListByUser(gomock.Any(), "user_1", 10, 0).
		Return([]*models.Job{}, 0, nil)

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
		ListByUser(gomock.Any(), "user_1", 50, 0).
		Return([]*models.Job{}, 0, nil)

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

func TestAddUserJob_created(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		Add(gomock.Any(), "user_1", int64(42)).
		Return(true, nil)

	body := `{"job_id":42}`
	req := httptest.NewRequest(http.MethodPost, "/user-jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.AddUserJob).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusCreated, w.Code)
	resp := bodyAsAddUserJob(t, w)
	assert.Equal(t, int64(42), resp.JobID)
	assert.True(t, resp.Created)
}

func TestAddUserJob_already_exists(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		Add(gomock.Any(), "user_1", int64(42)).
		Return(false, nil)

	body := `{"job_id":42}`
	req := httptest.NewRequest(http.MethodPost, "/user-jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.AddUserJob).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	resp := bodyAsAddUserJob(t, w)
	assert.Equal(t, int64(42), resp.JobID)
	assert.False(t, resp.Created)
}

func TestAddUserJob_unauthenticated(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	body := `{"job_id":42}`
	req := httptest.NewRequest(http.MethodPost, "/user-jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.AddUserJob).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAddUserJob_invalid_body(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	req := httptest.NewRequest(http.MethodPost, "/user-jobs", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.AddUserJob).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddUserJob_non_positive_job_id(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	body := `{"job_id":0}`
	req := httptest.NewRequest(http.MethodPost, "/user-jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.AddUserJob).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetHideStatus_hide(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		SetHideStatus(gomock.Any(), "user_1", int64(42), true).
		Return(nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/{job_id}/hide", httpx.HandlerFunc(h.SetHideStatus))

	body := strings.NewReader(`{"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/42/hide", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	resp := bodyAsSetHideStatus(t, w)
	assert.Equal(t, int64(42), resp.JobID)
	assert.True(t, resp.IsHidden)
}

func TestSetHideStatus_unhide(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		SetHideStatus(gomock.Any(), "user_1", int64(42), false).
		Return(nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/{job_id}/hide", httpx.HandlerFunc(h.SetHideStatus))

	body := strings.NewReader(`{"is_hidden":false}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/42/hide", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	resp := bodyAsSetHideStatus(t, w)
	assert.Equal(t, int64(42), resp.JobID)
	assert.False(t, resp.IsHidden)
}

func TestSetHideStatus_unauthenticated(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/{job_id}/hide", httpx.HandlerFunc(h.SetHideStatus))

	body := strings.NewReader(`{"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/42/hide", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSetHideStatus_invalid_job_id(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/{job_id}/hide", httpx.HandlerFunc(h.SetHideStatus))

	body := strings.NewReader(`{"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/abc/hide", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetHideStatus_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		SetHideStatus(gomock.Any(), "user_1", int64(42), true).
		Return(constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/{job_id}/hide", httpx.HandlerFunc(h.SetHideStatus))

	body := strings.NewReader(`{"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/42/hide", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusNotFound, w.Code)
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
	require.Len(t, routes, 3, "expected 3 user-jobs routes under /v1/user-jobs")
	assert.Equal(t, http.MethodGet, routes[0].Method)
	assert.Equal(t, "/v1/user-jobs", routes[0].Pattern)
	assert.Equal(t, http.MethodPost, routes[1].Method)
	assert.Equal(t, "/v1/user-jobs", routes[1].Pattern)
	assert.Equal(t, http.MethodPatch, routes[2].Method)
	assert.Equal(t, "/v1/user-jobs/{job_id}/hide", routes[2].Pattern)
}
