package jobs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucJobsMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs/mocks"
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
	job := m["Job"].(map[string]interface{})
	assert.Equal(t, slug, job["slug"])
	assert.Equal(t, true, m["Created"])
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
	job := m["Job"].(map[string]interface{})
	assert.Equal(t, slug, job["slug"])
	assert.Equal(t, false, m["Created"])
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

	expected := []*models.Job{{Slug: "job-1"}, {Slug: "job-2"}}
	mockJob.EXPECT().
		List(gomock.Any(), 20, "").
		Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]*models.Job]
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
		List(gomock.Any(), 20, "").
		Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]*models.Job]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Empty(t, resp.Data)
}

func TestListJobs_clerk_returns_owned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	expected := []*models.Job{{Slug: "job-1"}, {Slug: "job-2"}}
	mockJob.EXPECT().
		List(gomock.Any(), 20, "user_1").
		Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]*models.Job]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	got := resp.Data
	require.Len(t, got, 2)
	assert.Equal(t, "job-1", got[0].Slug)
}

func TestListJobs_api_key_returns_all(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	expected := []*models.Job{{Slug: "job-1"}, {Slug: "job-2"}}
	mockJob.EXPECT().
		List(gomock.Any(), 20, "").
		Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("api_key", "key_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[[]*models.Job]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	got := resp.Data
	require.Len(t, got, 2)
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
	mockJob.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	mockJob.EXPECT().SearchByCompany(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
}

func TestListJobs_clamped_limit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		List(gomock.Any(), 100, "user_1").
		Return([]*models.Job{}, nil)

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
		GetBySlug(gomock.Any(), slug).
		Return(expected, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /jobs/{slug}", httpx.HandlerFunc(h.GetJob))

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+slug, nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, slug, m["slug"])
}

func TestGetJob_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		GetBySlug(gomock.Any(), "missing").
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("GET /jobs/{slug}", httpx.HandlerFunc(h.GetJob))

	req := httptest.NewRequest(http.MethodGet, "/jobs/missing", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func strPtr(s string) *string {
	return &s
}

// ---------------------------------------------------------------------------
// UpdateJob — status-transition path
// ---------------------------------------------------------------------------

func TestUpdateJob_status_transition_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "some-job-new"
	updated := &models.Job{Slug: slug, Status: "analyze_pending"}
	mockJob.EXPECT().
		UpdateStatus(gomock.Any(), slug, "analyze_pending", (*string)(nil), (*string)(nil), "worker", "getojobu-worker").
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

func TestUpdateJob_status_transition_blocked_from_analyzed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	slug := "already-analyzed-job"
	mockJob.EXPECT().
		UpdateStatus(gomock.Any(), slug, "analyze_pending", (*string)(nil), (*string)(nil), "worker", "getojobu-worker").
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
	updated := &models.Job{Slug: slug, Status: "analyzed", AnalyzeResult: result}
	mockJob.EXPECT().
		UpdateStatus(gomock.Any(), slug, "analyzed", result, (*string)(nil), "worker", "getojobu-worker").
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
		UpdateStatus(gomock.Any(), slug, "failed", (*string)(nil), msg, "worker", "getojobu-worker").
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
		UpdateStatus(gomock.Any(), "missing", "analyze_pending", (*string)(nil), (*string)(nil), "worker", "getojobu-worker").
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
		Return([]string{"job-analyze-1", "job-analyze-2-force-abcdef123456"}, nil)

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
	assert.Equal(t, "job-analyze-2-force-abcdef123456", resp.Data.WorkflowIDs[1])
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
		Return(nil, constants.ErrNotFound)

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
		Return(nil, fmt.Errorf("%w: job is failed, force must be true", constants.ErrInvalidTransition))

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
