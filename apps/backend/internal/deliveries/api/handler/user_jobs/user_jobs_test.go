package user_jobs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucMatchingMock "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching/mocks"
	ucUserJobsMock "github.com/frasnym/getojobu/apps/backend/internal/usecase/user_jobs/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
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

	ujID := uuid.MustParse("ee46d5ea-893d-4951-a3d3-8c32d7459737")
	uID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	evalID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	evalStatus := "completed"
	eligibilityStatus := "eligible"
	recommendationLabel := "strong_match"
	reportStatus := "ready"
	resumeStatus := "generated"
	coverLetterStatus := "success"

	rows := []*models.UserJobWithJob{
		{
			UserJob: &models.UserJob{
				ID:                 ujID,
				UserID:             uID,
				JobID:              1,
				IsHidden:           false,
				LatestEvaluationID: &evalID,
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
			},
			Job: &models.Job{ID: 1, Slug: "job-1", Title: "Engineer"},
			Evaluation: &models.MatchingEvaluation{
				ID:                  evalID,
				UserJobID:           ujID,
				EvaluationStatus:    evalStatus,
				EligibilityStatus:   &eligibilityStatus,
				RecommendationLabel: &recommendationLabel,
				ReportStatus:        &reportStatus,
				ResumeStatus:        &resumeStatus,
				CoverLetterStatus:   &coverLetterStatus,
			},
		},
	}

	mockJobs.EXPECT().
		ListByUser(gomock.Any(), "user_1", models.UserJobFilter{}, 10, 0).
		Return(rows, 1, nil)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	jobs := bodyAsJobs(t, w)
	require.Len(t, jobs, 1)
	assert.Equal(t, ujID.String(), jobs[0]["id"])
	assert.Equal(t, false, jobs[0]["is_hidden"])

	evalMap := jobs[0]["evaluation"].(map[string]interface{})
	assert.Equal(t, evalID.String(), evalMap["id"])
	assert.Equal(t, "completed", evalMap["evaluation_status"])
	assert.Equal(t, "eligible", evalMap["eligibility_status"])
	assert.Equal(t, "strong_match", evalMap["recommendation_label"])
	assert.Equal(t, "ready", evalMap["report_status"])
	assert.Equal(t, "generated", evalMap["resume_status"])
	assert.Equal(t, "success", evalMap["cover_letter_status"])

	jobMap := jobs[0]["job"].(map[string]interface{})
	assert.Equal(t, float64(1), jobMap["id"])
	assert.Equal(t, "job-1", jobMap["slug"])
	assert.Equal(t, "Engineer", jobMap["title"])
}

func TestListUserJobs_withEvaluationFilters(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	expectedFilter := models.UserJobFilter{
		EvaluationStatus:    "completed",
		EligibilityStatus:   "eligible",
		RecommendationLabel: "strong_match",
		ReportStatus:        "ready",
		ResumeStatus:        "generated",
	}

	mockJobs.EXPECT().
		ListByUser(gomock.Any(), "user_1", expectedFilter, 10, 0).
		Return([]*models.UserJobWithJob{}, 0, nil)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs?evaluation_status=completed&eligibility_status=eligible&recommendation_label=strong_match&report_status=ready&resume_status=generated", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	jobs := bodyAsJobs(t, w)
	require.Len(t, jobs, 0)
}

func TestListUserJobs_rootIsUserJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	ujID := uuid.MustParse("ee46d5ea-893d-4951-a3d3-8c32d7459737")
	uID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ujCreatedAt := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	jobCreatedAt := time.Date(2026, 11, 15, 8, 0, 0, 0, time.UTC)

	rows := []*models.UserJobWithJob{
		{
			UserJob: &models.UserJob{
				ID:        ujID,
				UserID:    uID,
				JobID:     648,
				IsHidden:  false,
				CreatedAt: ujCreatedAt,
				UpdatedAt: ujCreatedAt,
			},
			Job: &models.Job{
				ID:        648,
				Slug:      "job-648",
				Title:     "DevOps",
				CreatedAt: jobCreatedAt,
				UpdatedAt: jobCreatedAt,
			},
		},
	}
	mockJobs.EXPECT().
		ListByUser(gomock.Any(), "user_1", models.UserJobFilter{}, 10, 0).
		Return(rows, 1, nil)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	jobs := bodyAsJobs(t, w)
	require.Len(t, jobs, 1)
	row := jobs[0]
	assert.Equal(t, ujID.String(), row["id"])
	assert.Equal(t, false, row["is_hidden"])
	assert.Equal(t, ujCreatedAt.Format(time.RFC3339), row["created_at"])
	assert.Nil(t, row["title"])
	assert.Nil(t, row["slug"])
	assert.Nil(t, row["is_owned"])
	assert.Nil(t, row["user_job_id"])

	job := row["job"].(map[string]interface{})
	assert.EqualValues(t, 648, job["id"])
	assert.Equal(t, "DevOps", job["title"])
}

func TestListUserJobs_preservesRowOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	id1 := uuid.New()
	id2 := uuid.New()
	rows := []*models.UserJobWithJob{
		{UserJob: &models.UserJob{ID: id1, JobID: 1}, Job: &models.Job{ID: 1, Title: "Job 1"}},
		{UserJob: &models.UserJob{ID: id2, JobID: 2}, Job: &models.Job{ID: 2, Title: "Job 2"}},
	}
	mockJobs.EXPECT().
		ListByUser(gomock.Any(), "user_1", models.UserJobFilter{}, 10, 0).
		Return(rows, 2, nil)

	req := httptest.NewRequest(http.MethodGet, "/user-jobs", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ListUserJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	jobs := bodyAsJobs(t, w)
	require.Len(t, jobs, 2)
	assert.Equal(t, id1.String(), jobs[0]["id"])
	assert.Equal(t, id2.String(), jobs[1]["id"])
}

func TestListUserJobs_empty(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	mockJobs.EXPECT().
		ListByUser(gomock.Any(), "user_1", models.UserJobFilter{}, 10, 0).
		Return([]*models.UserJobWithJob{}, 0, nil)

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
		ListByUser(gomock.Any(), "user_1", models.UserJobFilter{}, 50, 0).
		Return([]*models.UserJobWithJob{}, 0, nil)

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

func TestSetBulkHideStatus_hide(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	ujID := "11111111-1111-1111-1111-111111111111"

	mockJobs.EXPECT().
		SetBulkHideStatus(gomock.Any(), "user_1", []string{ujID}, true).
		Return(1, nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/bulk", httpx.HandlerFunc(h.SetBulkHideStatus))

	body := strings.NewReader(`{"user_job_ids":["` + ujID + `"],"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/bulk", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	resp := bodyAsSetHideStatus(t, w)
	assert.Equal(t, 1, resp.Updated)
}

func TestSetBulkHideStatus_unhide(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	ujID := "11111111-1111-1111-1111-111111111111"

	mockJobs.EXPECT().
		SetBulkHideStatus(gomock.Any(), "user_1", []string{ujID}, false).
		Return(1, nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/bulk", httpx.HandlerFunc(h.SetBulkHideStatus))

	body := strings.NewReader(`{"user_job_ids":["` + ujID + `"],"is_hidden":false}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/bulk", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	resp := bodyAsSetHideStatus(t, w)
	assert.Equal(t, 1, resp.Updated)
	assert.False(t, resp.Updated == 0)
}

func TestSetBulkHideStatus_multiple(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJobs := newHandlerWithMock(ctrl)

	id1 := "11111111-1111-1111-1111-111111111111"
	id2 := "22222222-2222-2222-2222-222222222222"

	mockJobs.EXPECT().
		SetBulkHideStatus(gomock.Any(), "user_1", []string{id1, id2}, true).
		Return(2, nil)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/bulk", httpx.HandlerFunc(h.SetBulkHideStatus))

	body := strings.NewReader(`{"user_job_ids":["` + id1 + `","` + id2 + `"],"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/bulk", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	resp := bodyAsSetHideStatus(t, w)
	assert.Equal(t, 2, resp.Updated)
}

func TestSetBulkHideStatus_empty_ids(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/bulk", httpx.HandlerFunc(h.SetBulkHideStatus))

	body := strings.NewReader(`{"user_job_ids":[],"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/bulk", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetBulkHideStatus_invalid_uuid(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/bulk", httpx.HandlerFunc(h.SetBulkHideStatus))

	body := strings.NewReader(`{"user_job_ids":["not-a-uuid"],"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/bulk", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetBulkHideStatus_unauthenticated(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PATCH /user-jobs/bulk", httpx.HandlerFunc(h.SetBulkHideStatus))

	body := strings.NewReader(`{"user_job_ids":["11111111-1111-1111-1111-111111111111"],"is_hidden":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/user-jobs/bulk", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusUnauthorized, w.Code)
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
	require.Len(t, routes, 4, "expected 4 user-jobs routes under /v1/user-jobs")
	assert.Equal(t, http.MethodGet, routes[0].Method)
	assert.Equal(t, "/v1/user-jobs", routes[0].Pattern)
	assert.Equal(t, http.MethodPost, routes[1].Method)
	assert.Equal(t, "/v1/user-jobs", routes[1].Pattern)
	assert.Equal(t, http.MethodPatch, routes[2].Method)
	assert.Equal(t, "/v1/user-jobs/bulk", routes[2].Pattern)
	assert.Equal(t, http.MethodGet, routes[3].Method)
	assert.Equal(t, "/v1/user-jobs/{user_job_id}/evaluation", routes[3].Pattern)
}

func TestGetEvaluation_includesJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockJobs := ucUserJobsMock.NewMockUserJobs(ctrl)
	mockMatching := ucMatchingMock.NewMockMatching(ctrl)
	h := &handler{
		cfg:        &config.Config{},
		ucJobs:     mockJobs,
		matchingUC: mockMatching,
	}

	uID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ujID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	evalID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	evalWithJob := &models.EvaluationWithJob{
		Evaluation: &models.MatchingEvaluation{
			ID:               evalID,
			UserJobID:        ujID,
			EvaluationStatus: "success",
			EvaluationData:   []byte(`{"score":85,"metadata":{"model":"test"}}`),
			ReportData:       []byte(`{"summary":"good","metadata":{"prompt":"v1"}}`),
		},
		Job: &models.Job{
			ID:            648,
			Slug:          "job-slug",
			Title:         "Platform Engineer",
			AnalyzeResult: json.RawMessage(`{"family":"technology","metadata":{"model":"test","prompt_version":"extract-job-v1"}}`),
		},
	}

	mockMatching.EXPECT().
		GetEvaluationByUserJobID(gomock.Any(), uID, ujID).
		Return(evalWithJob, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /user-jobs/{user_job_id}/evaluation", httpx.HandlerFunc(h.GetEvaluation))

	principal := principalCtx(uID.String())
	principal.FullName = "Jane Doe"
	req := httptest.NewRequest(http.MethodGet, "/user-jobs/"+ujID.String()+"/evaluation", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principal))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpx.Response[map[string]interface{}]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	data := resp.Data
	assert.Equal(t, evalID.String(), data["id"])
	assert.Equal(t, ujID.String(), data["user_job_id"])

	job := data["job"].(map[string]interface{})
	assert.EqualValues(t, 648, job["id"])
	assert.Equal(t, "job-slug", job["slug"])
	analyzeResult := job["analyze_result"].(map[string]interface{})
	assert.Equal(t, "technology", analyzeResult["family"])
	assert.Nil(t, analyzeResult["metadata"])

	evalObj := data["evaluation"].(map[string]interface{})
	assert.Equal(t, evalID.String(), evalObj["id"])
	assert.Equal(t, ujID.String(), evalObj["user_job_id"])
	assert.Equal(t, "success", evalObj["evaluation_status"])
	assert.Equal(t, "Jane Doe_Cover Letter_Platform Engineer.pdf", evalObj["cover_letter_file_name"])

	evalData := evalObj["evaluation_data"].(map[string]interface{})
	assert.Nil(t, evalData["metadata"])
	reportData := evalObj["report_data"].(map[string]interface{})
	assert.Nil(t, reportData["metadata"])
}

func TestGetEvaluation_notFoundForForeignLink(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockJobs := ucUserJobsMock.NewMockUserJobs(ctrl)
	mockMatching := ucMatchingMock.NewMockMatching(ctrl)
	h := &handler{
		cfg:        &config.Config{},
		ucJobs:     mockJobs,
		matchingUC: mockMatching,
	}

	uID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ujID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	mockMatching.EXPECT().
		GetEvaluationByUserJobID(gomock.Any(), uID, ujID).
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("GET /user-jobs/{user_job_id}/evaluation", httpx.HandlerFunc(h.GetEvaluation))

	req := httptest.NewRequest(http.MethodGet, "/user-jobs/"+ujID.String()+"/evaluation", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx(uID.String())))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp["job"])
}
