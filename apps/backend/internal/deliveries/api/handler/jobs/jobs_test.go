package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/dto"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucJobsMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs/mocks"
	ucMatchingMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching/mocks"
	ucUsersMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/users/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newHandlerWithMock(ctrl *gomock.Controller) (*handler, *ucJobsMocks.MockJob, *ucUsersMocks.MockUser) {
	mockJob := ucJobsMocks.NewMockJob(ctrl)
	mockUsers := ucUsersMocks.NewMockUser(ctrl)
	h := &handler{
		cfg:    &config.Config{},
		userUC: mockUsers,
		ucJobs: mockJob,
	}
	return h, mockJob, mockUsers
}

func newHandlerWithMatchingMock(ctrl *gomock.Controller) (*handler, *ucMatchingMocks.MockMatching) {
	mockJob := ucJobsMocks.NewMockJob(ctrl)
	mockUsers := ucUsersMocks.NewMockUser(ctrl)
	mockMatching := ucMatchingMocks.NewMockMatching(ctrl)
	h := &handler{
		cfg:        &config.Config{},
		userUC:     mockUsers,
		ucJobs:     mockJob,
		matchingUC: mockMatching,
	}
	return h, mockMatching
}

func principalCtx(actorType, actorID string) *models.Principal {
	return &models.Principal{ID: actorID, ActorType: actorType, ActorID: actorID}
}

func bodyAsMap(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp httpx.Response[map[string]interface{}]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	return resp.Data
}

func TestCreateJob_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "senior-backend-engineer-acme-jakarta"
	created := &models.CreateJobResult{
		Job:     &models.Job{Slug: slug, Status: "new"},
		Created: true,
	}
	mockJob.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(created, nil)

	body := strings.NewReader(`{"slug":"senior-backend-engineer-acme-jakarta","source":"linkedin","title":"Senior Backend Engineer","company":"Acme","location":"Jakarta"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	m := bodyAsMap(t, w)
	job := m["job"].(map[string]interface{})
	assert.Equal(t, slug, job["slug"])
	assert.Equal(t, true, m["created"])
}

func TestCreateJob_conflict_returns_created_false(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "senior-backend-engineer-acme-jakarta"
	existing := &models.CreateJobResult{
		Job:     &models.Job{Slug: slug, Status: "new"},
		Created: false,
	}
	mockJob.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(existing, nil)

	body := strings.NewReader(`{"slug":"senior-backend-engineer-acme-jakarta","source":"linkedin","title":"Senior Backend Engineer","company":"Acme","location":"Jakarta"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	job := m["job"].(map[string]interface{})
	assert.Equal(t, slug, job["slug"])
	assert.Equal(t, false, m["created"])
}

func TestCreateJob_clerk_rejects_body_user_ids(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"slug":"senior-backend-engineer-acme-jakarta","source":"linkedin","title":"Senior Backend Engineer","company":"Acme","location":"Jakarta","user_ids":["someone-else"]}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateJob_api_key_requires_user_ids(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"slug":"senior-backend-engineer-acme-jakarta","source":"linkedin","title":"Senior Backend Engineer","company":"Acme","location":"Jakarta"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("api_key", "key_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateJob_api_key_with_user_ids(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "senior-backend-engineer-acme-jakarta"
	created := &models.CreateJobResult{
		Job:     &models.Job{Slug: slug, Status: "new"},
		Created: true,
	}
	mockJob.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(created, nil)

	body := strings.NewReader(`{"slug":"senior-backend-engineer-acme-jakarta","source":"linkedin","title":"Senior Backend Engineer","company":"Acme","location":"Jakarta","user_ids":["user_42"]}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("api_key", "key_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateJob_unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"slug":"x","source":"linkedin","title":"T","company":"C","location":"L"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateJob_validation_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)

	body := strings.NewReader(`{"source":"","title":"T","company":"C","location":"L"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateJob_generates_slug_internally(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	expectedSlug := "senior-backend-engineer-acme-jakarta"
	mockJob.EXPECT().
		Create(gomock.Any(), gomock.Cond(func(x any) bool {
			params, ok := x.(models.CreateJobParams)
			return ok && params.Slug == expectedSlug
		})).
		Return(&models.CreateJobResult{
			Job:     &models.Job{Slug: expectedSlug, Status: "new"},
			Created: true,
		}, nil)

	// Body without slug - slug is generated internally from title, company, location
	body := strings.NewReader(`{"source":"linkedin","title":"Senior Backend Engineer","company":"Acme","location":"Jakarta"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateJob_malformed_body(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockJob.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)
}

func TestBulkCreateJob_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	results := []models.BulkCreateJobResult{
		{Slug: "job-1", Created: true},
		{Slug: "job-2", Created: false},
	}
	mockJob.EXPECT().
		CreateBulk(gomock.Any(), gomock.Any()).
		Return(results, nil)

	body := strings.NewReader(`{"jobs":[{"slug":"job-1","source":"linkedin","title":"Job 1","company":"Acme","location":"Jakarta","user_ids":["user_42"]},{"slug":"job-2","source":"linkedin","title":"Job 2","company":"Acme","location":"Jakarta","user_ids":["user_43"]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkCreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]models.BulkCreateJobResult]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	got := resp.Data
	require.Len(t, got, 2)
	assert.Equal(t, "job-1", got[0].Slug)
	assert.True(t, got[0].Created)
	assert.Equal(t, "job-2", got[1].Slug)
	assert.False(t, got[1].Created)
}

func TestBulkCreateJob_empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"jobs":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkCreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockJob.EXPECT().CreateBulk(gomock.Any(), gomock.Any()).Times(0)
}

func TestListJobs_default(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	expected := []*models.JobWithOwnership{
		{Job: &models.Job{Slug: "job-1"}},
		{Job: &models.Job{Slug: "job-2"}},
	}
	mockJob.EXPECT().
		List(gomock.Any(), 10, 0, "").
		Return(expected, 2, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]dto.JobResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	got := resp.Data
	require.Len(t, got, 2)
}

func TestListJobs_empty_returns_empty_array(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		List(gomock.Any(), 10, 0, "").
		Return(nil, 0, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]dto.JobResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Empty(t, resp.Data)
}

func TestListJobs_returns_all_jobs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	expected := []*models.JobWithOwnership{
		{Job: &models.Job{Slug: "job-1"}, IsOwned: true},
		{Job: &models.Job{Slug: "job-2"}, IsOwned: false},
	}
	mockJob.EXPECT().
		List(gomock.Any(), 10, 0, "user_1").
		Return(expected, 2, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]dto.JobResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	got := resp.Data
	require.Len(t, got, 2)
	assert.Equal(t, "job-1", got[0].Slug)
	assert.True(t, *got[0].IsOwned)
	assert.False(t, *got[1].IsOwned)
}

func TestListJobs_api_key_returns_all(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	expected := []*models.JobWithOwnership{
		{Job: &models.Job{Slug: "job-1"}},
		{Job: &models.Job{Slug: "job-2"}},
	}
	mockJob.EXPECT().
		List(gomock.Any(), 10, 0, "").
		Return(expected, 2, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("api_key", "key_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]dto.JobResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	got := resp.Data
	require.Len(t, got, 2)
	assert.Nil(t, got[0].IsOwned)
}

func TestListJobs_invalid_limit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/jobs?limit=abc", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockJob.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
}

func TestListJobs_clamped_limit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		List(gomock.Any(), 50, 0, "user_1").
		Return([]*models.JobWithOwnership{}, 0, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs?limit=500", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetJob_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "golang-developer-google-singapore"
	expected := &models.Job{Slug: slug, Status: "new"}
	mockJob.EXPECT().
		GetBySlug(gomock.Any(), slug, "").
		Return(&models.JobWithOwnership{Job: expected}, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /jobs/{slug}", httpx.HandlerFunc(h.GetJob))

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+slug, nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, slug, m["slug"])
	assert.Nil(t, m["is_owned"])
}

func TestGetJob_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		GetBySlug(gomock.Any(), "missing", "").
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("GET /jobs/{slug}", httpx.HandlerFunc(h.GetJob))

	req := httptest.NewRequest(http.MethodGet, "/jobs/missing", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetJob_clerk_sees_any_job(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "golang-developer-google-singapore"
	expected := &models.Job{Slug: slug, Status: "new"}
	mockJob.EXPECT().
		GetBySlug(gomock.Any(), slug, "user_1").
		Return(&models.JobWithOwnership{Job: expected, IsOwned: false}, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /jobs/{slug}", httpx.HandlerFunc(h.GetJob))

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+slug, nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, slug, m["slug"])
	assert.Equal(t, false, m["is_owned"])
	assert.Nil(t, m["user_job"])
}

func TestGetJob_includesUserJobForOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "golang-developer-google-singapore"
	ujID := uuid.MustParse("ee46d5ea-893d-4951-a3d3-8c32d7459737")
	expected := &models.Job{Slug: slug, Status: "new"}
	mockJob.EXPECT().
		GetBySlug(gomock.Any(), slug, "user_1").
		Return(&models.JobWithOwnership{Job: expected, IsOwned: true, UserJobID: &ujID}, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /jobs/{slug}", httpx.HandlerFunc(h.GetJob))

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+slug, nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, true, m["is_owned"])
	ujMap, ok := m["user_job"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, ujID.String(), ujMap["id"])
	assert.Nil(t, m["is_hidden"])
	assert.Nil(t, m["user_job_id"])
}

func strPtr(s string) *string {
	return &s
}

// ---------------------------------------------------------------------------
// UpdateJob — status-transition path
// ---------------------------------------------------------------------------

func TestRoute_registers_manual_endpoints_under_jobs_group(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)
	rt := httpx.New()
	h.Route(rt.Group("/v1/jobs"))

	patterns := map[string]bool{}
	for _, route := range rt.Routes() {
		if route.Method == http.MethodPost {
			patterns[route.Pattern] = true
		}
	}
	assert.True(t, patterns["/v1/jobs/manual"], "missing POST /v1/jobs/manual")
	assert.True(t, patterns["/v1/jobs/scrape-url"], "missing POST /v1/jobs/scrape-url")
}

func TestCreateManualJob_missing_title_returns_400(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("POST /jobs/manual", httpx.HandlerFunc(h.CreateManualJob))

	req := httptest.NewRequest(http.MethodPost, "/jobs/manual", strings.NewReader(`{"company":"Acme","location":"Berlin"}`))
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), &models.Principal{ID: uuid.NewString()}))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateJob_status_transition_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "some-job-new"
	updated := &models.Job{Slug: slug, Status: "analyze_pending"}
	mockJob.EXPECT().
		UpdateStatus(gomock.Any(), models.UpdateJobParams{Slug: slug, Status: "analyze_pending", AnalyzeResult: (*string)(nil), FailureMessage: (*string)(nil), UpdatedByType: "worker", UpdatedByID: "getojobu-worker"}).
		Return(updated, nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJob))

	body := strings.NewReader(`{"status":"analyze_pending"}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/"+slug, body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("worker", "getojobu-worker")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "analyze_pending", m["status"])
}

func TestUpdateJob_clerk_caller_cannot_touch_other_users_private_job(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "m-personal-job"
	mockJob.EXPECT().
		GetBySlug(gomock.Any(), slug, "user-a").
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJob))

	req := httptest.NewRequest(http.MethodPatch, "/jobs/"+slug, strings.NewReader(`{"status":"new"}`))
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), &models.Principal{ID: "user-a"}))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateJob_status_transition_blocked_from_analyzed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "already-analyzed-job"
	mockJob.EXPECT().
		UpdateStatus(gomock.Any(), models.UpdateJobParams{Slug: slug, Status: "analyze_pending", AnalyzeResult: (*string)(nil), FailureMessage: (*string)(nil), UpdatedByType: "worker", UpdatedByID: "getojobu-worker"}).
		Return(nil, fmt.Errorf("%w: %s", constants.ErrInvalidTransition, "job is already finalized"))

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJob))

	body := strings.NewReader(`{"status":"analyze_pending"}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/"+slug, body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("worker", "getojobu-worker")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateJob_status_transition_with_analyze_result(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "pending-job"
	result := strPtr(`{"title":"Engineer"}`)
	updated := &models.Job{Slug: slug, Status: "analyzed", AnalyzeResult: json.RawMessage(*result)}
	mockJob.EXPECT().
		UpdateStatus(gomock.Any(), models.UpdateJobParams{Slug: slug, Status: "analyzed", AnalyzeResult: result, FailureMessage: (*string)(nil), UpdatedByType: "worker", UpdatedByID: "getojobu-worker"}).
		Return(updated, nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJob))

	body := strings.NewReader(`{"status":"analyzed","analyze_result":"{\"title\":\"Engineer\"}"}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/"+slug, body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("worker", "getojobu-worker")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateJob_status_transition_failed_with_message(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "pending-job-fail"
	msg := strPtr("LLM timeout")
	updated := &models.Job{Slug: slug, Status: "failed", FailureMessage: msg}
	mockJob.EXPECT().
		UpdateStatus(gomock.Any(), models.UpdateJobParams{Slug: slug, Status: "failed", AnalyzeResult: (*string)(nil), FailureMessage: msg, UpdatedByType: "worker", UpdatedByID: "getojobu-worker"}).
		Return(updated, nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJob))

	body := strings.NewReader(`{"status":"failed","failure_message":"LLM timeout"}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/"+slug, body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("worker", "getojobu-worker")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateJob_status_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		UpdateStatus(gomock.Any(), models.UpdateJobParams{Slug: "missing", Status: "analyze_pending", AnalyzeResult: (*string)(nil), FailureMessage: (*string)(nil), UpdatedByType: "worker", UpdatedByID: "getojobu-worker"}).
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJob))

	body := strings.NewReader(`{"status":"analyze_pending"}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/missing", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("worker", "getojobu-worker")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateJob_status_empty_string_rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJob))

	body := strings.NewReader(`{"status":""}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/some-job", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("worker", "getojobu-worker")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// BulkQueueAnalyzeJob
// ---------------------------------------------------------------------------

func TestBulkQueueAnalyzeJob_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		QueueAnalysisBulk(gomock.Any(), []models.QueueAnalysisItem{
			{Slug: "job-1", Force: false},
			{Slug: "job-2", Force: true},
		}, gomock.Any()).
		Return(models.QueueAnalysisBulkResult{WorkflowIDs: []string{"job-analyze-1", "job-analyze-2"}}, nil)

	body := strings.NewReader(`[{"slug":"job-1","force":false},{"slug":"job-2","force":true}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk-queue-analyze", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkQueueAnalyzeJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp httpx.Response[bulkQueueAnalyzeResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data.WorkflowIDs, 2)
	assert.Equal(t, "job-analyze-1", resp.Data.WorkflowIDs[0])
	assert.Equal(t, "job-analyze-2", resp.Data.WorkflowIDs[1])
}

func TestBulkQueueAnalyzeJob_partial_failure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		QueueAnalysisBulk(gomock.Any(), []models.QueueAnalysisItem{
			{Slug: "job-no-desc", Force: true},
			{Slug: "job-ok", Force: true},
		}, gomock.Any()).
		Return(models.QueueAnalysisBulkResult{
			WorkflowIDs: []string{"job-analyze-2"},
			Failed: []models.QueueAnalysisFailure{
				{Slug: "job-no-desc", Error: `job "job-no-desc" has no description to analyze`},
			},
		}, nil)

	body := strings.NewReader(`[{"slug":"job-no-desc","force":true},{"slug":"job-ok","force":true}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk-queue-analyze", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkQueueAnalyzeJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp httpx.Response[bulkQueueAnalyzeResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Equal(t, []string{"job-analyze-2"}, resp.Data.WorkflowIDs)
	require.Len(t, resp.Data.Failed, 1)
	assert.Equal(t, "job-no-desc", resp.Data.Failed[0].Slug)
	assert.Equal(t, `job "job-no-desc" has no description to analyze`, resp.Data.Failed[0].Error)
}

func TestBulkQueueAnalyzeJob_empty_array(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`[]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk-queue-analyze", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkQueueAnalyzeJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkQueueAnalyzeJob_missing_slug(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`[{"slug":"","force":false}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk-queue-analyze", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkQueueAnalyzeJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkQueueAnalyzeJob_unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`[{"slug":"job-1","force":false}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk-queue-analyze", body)
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkQueueAnalyzeJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestBulkQueueAnalyzeJob_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		QueueAnalysisBulk(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(models.QueueAnalysisBulkResult{}, constants.ErrNotFound)

	body := strings.NewReader(`[{"slug":"job-missing","force":false}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk-queue-analyze", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkQueueAnalyzeJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBulkQueueAnalyzeJob_invalid_transition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		QueueAnalysisBulk(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(models.QueueAnalysisBulkResult{}, fmt.Errorf("%w: job is failed, force must be true", constants.ErrInvalidTransition))

	body := strings.NewReader(`[{"slug":"job-failed","force":false}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk-queue-analyze", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkQueueAnalyzeJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkQueueAnalyzeJob_invalid_force_type(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`[{"slug":"job-1","force":"not-a-bool"}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk-queue-analyze", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkQueueAnalyzeJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateJobRequest_Validate(t *testing.T) {
	req := createJobRequest{
		Source:   "glassdoor",
		Title:    "Go Engineer",
		Company:  "Tech Corp",
		Location: "Norway",
	}

	// YYYY-MM-DD
	req.PostedDate = "2026-11-12"
	parsed, err := req.Validate()
	require.NoError(t, err)
	assert.Equal(t, 2026, parsed.Year())
	assert.Equal(t, time.November, parsed.Month())
	assert.Equal(t, 12, parsed.Day())

	// RFC3339
	req.PostedDate = "2026-11-12T10:00:00Z"
	parsed, err = req.Validate()
	require.NoError(t, err)
	assert.Equal(t, 2026, parsed.Year())

	// RFC3339Nano / ISO format from Python datetime
	req.PostedDate = "2026-11-12T10:00:00.123456+00:00"
	parsed, err = req.Validate()
	require.NoError(t, err)
	assert.Equal(t, 2026, parsed.Year())

	// Invalid format
	req.PostedDate = "invalid-date"
	_, err = req.Validate()
	require.Error(t, err)
}

func TestBulkQueueAnalyzeRequest_Validate(t *testing.T) {
	t.Run("empty slice returns error", func(t *testing.T) {
		req := bulkQueueAnalyzeRequest{}
		err := req.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one job is required")
	})

	t.Run("over 100 items returns error", func(t *testing.T) {
		req := make(bulkQueueAnalyzeRequest, 101)
		for i := range req {
			req[i] = bulkQueueAnalyzeItemRequest{Slug: fmt.Sprintf("job-%d", i)}
		}
		err := req.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maximum 100 jobs per request")
	})

	t.Run("valid items return nil", func(t *testing.T) {
		req := bulkQueueAnalyzeRequest{
			{Slug: "job-1", Force: false},
			{Slug: "job-2", Force: true},
		}
		err := req.Validate()
		require.NoError(t, err)
	})
}

func TestCreateEvaluationRequest_Validate(t *testing.T) {
	t.Run("empty slice returns error", func(t *testing.T) {
		req := createEvaluationRequest{}
		err := req.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one job is required")
	})

	t.Run("over 100 items returns error", func(t *testing.T) {
		req := make(createEvaluationRequest, 101)
		for i := range req {
			req[i] = createEvaluationItemRequest{Slug: fmt.Sprintf("job-%d", i)}
		}
		err := req.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maximum 100 jobs per request")
	})

	t.Run("valid items return nil", func(t *testing.T) {
		req := createEvaluationRequest{
			{Slug: "job-1"},
			{Slug: "job-2", UserIDs: []string{"user_42"}},
		}
		err := req.Validate()
		require.NoError(t, err)
	})
}

// ---------------------------------------------------------------------------
// CreateEvaluation — insert-only evaluation creation
// ---------------------------------------------------------------------------

func TestCreateEvaluation_clerk_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockMatching := newHandlerWithMatchingMock(ctrl)

	id := uuid.New()
	mockMatching.EXPECT().
		CreateEvaluations(gomock.Any(), []models.CreateEvaluationItem{{Slug: "job-1"}}, gomock.Any()).
		Return([]*models.MatchingEvaluationSummary{{
			Slug: "job-1", ID: &id, EvaluationStatus: "new",
		}}, nil)

	body := strings.NewReader(`[{"slug":"job-1"}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/evaluation", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateEvaluation).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp httpx.Response[[]models.MatchingEvaluationSummary]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "new", resp.Data[0].EvaluationStatus)
}

func TestCreateEvaluation_clerk_rejects_body_user_ids(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMatchingMock(ctrl)

	body := strings.NewReader(`[{"slug":"job-1","user_ids":["user_42"]}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/evaluation", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateEvaluation).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateEvaluation_api_key_passes_user_ids(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockMatching := newHandlerWithMatchingMock(ctrl)

	id := uuid.New()
	mockMatching.EXPECT().
		CreateEvaluations(gomock.Any(), []models.CreateEvaluationItem{
			{Slug: "job-1", UserIDs: []string{"user_42"}},
			{Slug: "job-2", UserIDs: []string{"user_43"}},
		}, gomock.Any()).
		Return([]*models.MatchingEvaluationSummary{
			{Slug: "job-1", ID: &id, EvaluationStatus: "new"},
			{Slug: "job-2", ID: &id, EvaluationStatus: "new"},
		}, nil)

	body := strings.NewReader(`[{"slug":"job-1","user_ids":["user_42"]},{"slug":"job-2","user_ids":["user_43"]}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/evaluation", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("api_key", "key_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateEvaluation).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestCreateEvaluation_api_key_without_user_ids_defers_resolution_to_usecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockMatching := newHandlerWithMatchingMock(ctrl)

	id := uuid.New()
	mockMatching.EXPECT().
		CreateEvaluations(gomock.Any(), []models.CreateEvaluationItem{{Slug: "job-1"}}, gomock.Any()).
		Return([]*models.MatchingEvaluationSummary{{
			Slug: "job-1", ID: &id, UserID: "user_42", EvaluationStatus: "new",
		}}, nil)

	body := strings.NewReader(`[{"slug":"job-1"}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/evaluation", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("api_key", "key_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateEvaluation).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp httpx.Response[[]models.MatchingEvaluationSummary]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "user_42", resp.Data[0].UserID)
}

func TestCreateEvaluation_empty_array(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMatchingMock(ctrl)

	body := strings.NewReader(`[]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/evaluation", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateEvaluation).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateEvaluation_over_100_items(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMatchingMock(ctrl)

	items := make([]createEvaluationItemRequest, 101)
	for i := range items {
		items[i] = createEvaluationItemRequest{Slug: fmt.Sprintf("job-%d", i)}
	}
	body, err := json.Marshal(items)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/jobs/evaluation", bytes.NewReader(body))
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateEvaluation).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateEvaluation_partial_failure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockMatching := newHandlerWithMatchingMock(ctrl)

	id := uuid.New()
	errStr := "job not found"
	mockMatching.EXPECT().
		CreateEvaluations(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*models.MatchingEvaluationSummary{
			{Slug: "job-1", ID: &id, EvaluationStatus: "new"},
			{Slug: "job-2", Error: &errStr},
		}, nil)

	body := strings.NewReader(`[{"slug":"job-1"},{"slug":"job-2"}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/evaluation", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateEvaluation).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp httpx.Response[[]models.MatchingEvaluationSummary]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
	assert.Equal(t, "job-1", resp.Data[0].Slug)
	assert.Nil(t, resp.Data[0].Error)
	assert.Equal(t, "job-2", resp.Data[1].Slug)
	require.NotNil(t, resp.Data[1].Error)
	assert.Equal(t, "job not found", *resp.Data[1].Error)
}

func TestCreateEvaluation_unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMatchingMock(ctrl)

	body := strings.NewReader(`[{"slug":"job-1"}]`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/evaluation", body)
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateEvaluation).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
