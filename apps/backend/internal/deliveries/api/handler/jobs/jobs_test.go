package jobs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
	"github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newHandlerWithMock(ctrl *gomock.Controller) (*handler, *mocks.MockJob) {
	mockJob := mocks.NewMockJob(ctrl)
	h := &handler{
		cfg:    &config.Config{},
		ucJobs: mockJob,
	}
	return h, mockJob
}

func principalCtx(actorType, actorID string) *models.Principal {
	return &models.Principal{ID: actorID, ActorType: actorType, ActorID: actorID}
}

func bodyAsMap(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestCreateJob_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

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
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
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
	h, mockJob := newHandlerWithMock(ctrl)

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
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	job := m["Job"].(map[string]interface{})
	assert.Equal(t, slug, job["slug"])
	assert.Equal(t, false, m["Created"])
}

func TestCreateJob_unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"slug":"x","source":"linkedin","title":"T","company":"C","location":"L"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateJob_validation_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"slug":"","source":"linkedin","title":"T","company":"C","location":"L"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.CreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockJob.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)
}

func TestCreateJob_malformed_body(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

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
	h, mockJob := newHandlerWithMock(ctrl)

	results := []models.BulkCreateJobResult{
		{Slug: "job-1", Created: true},
		{Slug: "job-2", Created: false},
	}
	mockJob.EXPECT().
		CreateBulk(gomock.Any(), gomock.Any()).
		Return(results, nil)

	body := strings.NewReader(`{"jobs":[{"slug":"job-1","source":"linkedin","title":"Job 1","company":"Acme","location":"Jakarta"},{"slug":"job-2","source":"linkedin","title":"Job 2","company":"Acme","location":"Jakarta"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs/bulk", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.BulkCreateJob).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []models.BulkCreateJobResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "job-1", got[0].Slug)
	assert.True(t, got[0].Created)
	assert.Equal(t, "job-2", got[1].Slug)
	assert.False(t, got[1].Created)
}

func TestBulkCreateJob_empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

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
	h, mockJob := newHandlerWithMock(ctrl)

	expected := []*models.Job{{Slug: "job-1"}, {Slug: "job-2"}}
	mockJob.EXPECT().
		ListRecent(gomock.Any(), 20).
		Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []*models.Job
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
}

func TestListJobs_empty_returns_empty_array(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		ListRecent(gomock.Any(), 20).
		Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `[]`, strings.TrimSpace(w.Body.String()))
}

func TestListJobs_with_company_filter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

	expected := []*models.Job{{Slug: "job-1", Company: "Acme Corp"}}
	mockJob.EXPECT().
		SearchByCompany(gomock.Any(), "acme", 20).
		Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/jobs?company=acme", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []*models.Job
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, "Acme Corp", got[0].Company)
}

func TestListJobs_invalid_limit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/jobs?limit=abc", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockJob.EXPECT().ListRecent(gomock.Any(), gomock.Any()).Times(0)
	mockJob.EXPECT().SearchByCompany(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
}

func TestGetJob_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

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
	h, mockJob := newHandlerWithMock(ctrl)

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

func TestUpdateJobDescription_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

	slug := "update-null-desc-jakarta"
	updated := &models.Job{Slug: slug, Description: strPtr("New description.")}
	mockJob.EXPECT().
		UpdateDescription(gomock.Any(), slug, "New description.", "cron", "cron-scrape").
		Return(updated, nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJobDescription))

	body := strings.NewReader(`{"description":"New description."}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/"+slug, body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "New description.", m["description"])
}

func TestUpdateJobDescription_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		UpdateDescription(gomock.Any(), "missing", "New description.", "user", "user_1").
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJobDescription))

	body := strings.NewReader(`{"description":"New description."}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/missing", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user", "user_1")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateJobDescription_blocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob := newHandlerWithMock(ctrl)

	slug := "update-blocked-jakarta"
	mockJob.EXPECT().
		UpdateDescription(gomock.Any(), slug, "New description.", "cron", "cron-scrape").
		Return(nil, ucJobs.ErrDescriptionBlocked)

	mux := http.NewServeMux()
	mux.Handle("PATCH /jobs/{slug}", httpx.HandlerFunc(h.UpdateJobDescription))

	body := strings.NewReader(`{"description":"New description."}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/"+slug, body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUpdateJobDescription_unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"description":"New description."}`)
	req := httptest.NewRequest(http.MethodPatch, "/jobs/slug", body)
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.UpdateJobDescription).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func strPtr(s string) *string {
	return &s
}
