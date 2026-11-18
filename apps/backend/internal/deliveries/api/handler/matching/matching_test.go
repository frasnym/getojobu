package matching_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	contractUsecase "github.com/frasnym/getojobu/apps/backend/internal/contract/usecase"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/dto"
	handlerJobs "github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/jobs"
	handlerMatching "github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/matching"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	usecaseMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	usecaseMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching/mocks"
	userMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/users/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

func setupTestRouter(
	ctrl *gomock.Controller,
	mockMatching *usecaseMocks.MockMatching,
	mockUsers *userMocks.MockUser,
	apiKey string,
) *httpx.Router {
	cfg := &config.Config{
		MachineAuth: config.MachineAuth{
			APIKeys: map[string]config.MachineKey{
				apiKey: {ActorType: "worker", ActorID: "worker-1"},
			},
		},
	}

	c := &contract.Contract{
		Cfg: cfg,
		Usecase: &contractUsecase.Contract{
			Matching: mockMatching,
			Users:    mockUsers,
		},
	}

	rt := httpx.New()
	v1 := rt.Group("/v1")
	handlerJobs.NewHandler(c).Route(v1.Group("/jobs"))
	handlerMatching.NewHandler(c).Route(v1.Group("/matching-evaluations"))

	return rt
}

func TestBulkAnalyzeCompatibilityHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMatching := usecaseMocks.NewMockMatching(ctrl)
	mockUsers := userMocks.NewMockUser(ctrl)
	apiKey := "test-worker-key"

	rt := setupTestRouter(ctrl, mockMatching, mockUsers, apiKey)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`[{"slug":"job-1","force":false}]`)
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/bulk-analyze-compatibility", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("empty array returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`[]`)
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/bulk-analyze-compatibility", body)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("valid bulk request returns 202 with partial results", func(t *testing.T) {
		newID := uuid.New()
		errStr := "already evaluated"
		mockMatching.EXPECT().TriggerCompatibilityBulk(gomock.Any(), []models.BulkAnalyzeCompatibilityItem{
			{Slug: "job-1", Force: false},
			{Slug: "job-2", Force: true},
		}, gomock.Any()).Return([]*models.MatchingEvaluationSummary{
			{
				Slug:             "job-1",
				ID:               &newID,
				EvaluationStatus: string(constants.MatchingStatusQueued),
			},
			{
				Slug:  "job-2",
				Error: &errStr,
			},
		}, nil)

		body := bytes.NewBufferString(`[{"slug":"job-1","force":false},{"slug":"job-2","force":true}]`)
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/bulk-analyze-compatibility", body)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusAccepted, rec.Code)

		var resp httpx.Response[[]models.MatchingEvaluationSummary]
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Len(t, resp.Data, 2)
		assert.Equal(t, "job-1", resp.Data[0].Slug)
		assert.Nil(t, resp.Data[0].Error)
		assert.Equal(t, "job-2", resp.Data[1].Slug)
		assert.Equal(t, "already evaluated", *resp.Data[1].Error)
	})
}

func TestUpdateEvaluationHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMatching := usecaseMocks.NewMockMatching(ctrl)
	mockUsers := userMocks.NewMockUser(ctrl)
	apiKey := "test-worker-key"

	rt := setupTestRouter(ctrl, mockMatching, mockUsers, apiKey)
	evalID := uuid.New()

	t.Run("worker updates evaluation to pending", func(t *testing.T) {
		mockMatching.EXPECT().UpdateEvaluation(gomock.Any(), evalID, gomock.Any()).Return(&models.MatchingEvaluation{
			ID:               evalID,
			EvaluationStatus: string(constants.MatchingStatusPending),
		}, nil)

		body, err := json.Marshal(map[string]interface{}{
			"evaluation_status": "pending",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String(), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing api key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String(), nil)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid transition returns 400", func(t *testing.T) {
		mockMatching.EXPECT().UpdateEvaluation(gomock.Any(), evalID, gomock.Any()).Return(
			nil, fmt.Errorf("%w: transition from pending to pending is not allowed", constants.ErrInvalidTransition),
		)

		body, err := json.Marshal(map[string]interface{}{
			"evaluation_status": "pending",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String(), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestReportHandlers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMatching := usecaseMocks.NewMockMatching(ctrl)
	mockUsers := userMocks.NewMockUser(ctrl)
	apiKey := "test-worker-key"

	rt := setupTestRouter(ctrl, mockMatching, mockUsers, apiKey)
	evalID := uuid.New()

	t.Run("worker updates report", func(t *testing.T) {
		repStatus := string(constants.ReportStatusSuccess)
		mockMatching.EXPECT().UpdateEvaluationReport(gomock.Any(), evalID, gomock.Any()).Return(&models.MatchingEvaluation{
			ID:               evalID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
			ReportStatus:     &repStatus,
			ReportData:       json.RawMessage(`{"job_title":"Senior Dev"}`),
		}, nil)

		body, err := json.Marshal(map[string]interface{}{
			"report_status": "success",
			"report_data":   map[string]string{"job_title": "Senior Dev"},
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String()+"/report", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("worker updates report to queued_retry", func(t *testing.T) {
		repStatus := string(constants.ReportStatusQueuedRetry)
		mockMatching.EXPECT().UpdateEvaluationReport(gomock.Any(), evalID, gomock.Any()).Return(&models.MatchingEvaluation{
			ID:               evalID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
			ReportStatus:     &repStatus,
		}, nil)

		body, err := json.Marshal(map[string]interface{}{
			"report_status": "queued_retry",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String()+"/report", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("worker updates report to queued_retry", func(t *testing.T) {
		repStatus := string(constants.ReportStatusQueuedRetry)
		mockMatching.EXPECT().UpdateEvaluationReport(gomock.Any(), evalID, gomock.Any()).Return(&models.MatchingEvaluation{
			ID:               evalID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
			ReportStatus:     &repStatus,
		}, nil)

		body, err := json.Marshal(map[string]interface{}{
			"report_status": "queued_retry",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String()+"/report", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("bulk queue generate report accepts batch", func(t *testing.T) {
		mockMatching.EXPECT().BulkQueueGenerateReport(gomock.Any(), []models.BulkGenerateReportItem{
			{ID: evalID, Force: false},
		}, gomock.Any()).Return([]*models.MatchingEvaluationSummary{
			{
				ID:               &evalID,
				EvaluationStatus: string(constants.MatchingStatusSuccess),
			},
		}, nil)

		body, err := json.Marshal([]map[string]interface{}{
			{"id": evalID.String(), "force": false},
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/matching-evaluations/bulk-generate-report", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusAccepted, rec.Code)
	})

	t.Run("update report rejects invalid report status", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"report_status": "invalid_status",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String()+"/report", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("update report rejects success status with empty report_data", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"report_status": "success",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String()+"/report", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("bulk queue generate report returns 401 when unauthenticated", func(t *testing.T) {
		body, err := json.Marshal([]map[string]interface{}{
			{"id": evalID.String(), "force": false},
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/matching-evaluations/bulk-generate-report", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("update resume returns 401 when unauthenticated", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"resume_status": "success",
		})
		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String()+"/resume", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("update resume updates status and returns 200", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"resume_status": "success",
			"resume_artifacts": map[string]interface{}{
				"pdf_key":     "resumes/resume.pdf",
				"content_key": "resumes/resume.json",
			},
		})
		pdfKey := "resumes/resume.pdf"
		jsonKey := "resumes/resume.json"
		resStatus := "success"
		mockMatching.EXPECT().UpdateEvaluationResume(gomock.Any(), evalID, gomock.Cond(func(x any) bool {
			req, ok := x.(usecaseMatching.UpdateEvaluationResumeRequest)
			return ok && req.ResumeStatus == "success" && req.ResumePDFKey != nil && *req.ResumePDFKey == pdfKey && req.ResumeJSONKey != nil && *req.ResumeJSONKey == jsonKey
		})).Return(&models.MatchingEvaluation{
			ID:            evalID,
			ResumeStatus:  &resStatus,
			ResumePDFKey:  &pdfKey,
			ResumeJSONKey: &jsonKey,
		}, nil)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String()+"/resume", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("get resume returns 401 when unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/matching-evaluations/"+evalID.String()+"/resume", nil)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("get resume returns 200 with metadata", func(t *testing.T) {
		resStatus := "success"
		mockMatching.EXPECT().GetResume(gomock.Any(), evalID, gomock.Any()).Return(&models.MatchingEvaluation{
			ID:           evalID,
			ResumeStatus: &resStatus,
		}, nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/matching-evaluations/"+evalID.String()+"/resume", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("bulk queue generate resume returns 401 when unauthenticated", func(t *testing.T) {
		body, _ := json.Marshal([]map[string]interface{}{
			{"id": evalID.String(), "force": false},
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/matching-evaluations/bulk-generate-resume", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("bulk queue generate resume accepts items with force and returns 202", func(t *testing.T) {
		body, _ := json.Marshal([]map[string]interface{}{
			{"id": evalID.String(), "force": true},
		})
		resumeStatus := string(constants.ResumeStatusSuccess)
		mockMatching.EXPECT().BulkQueueGenerateResume(gomock.Any(), []models.BulkGenerateResumeItem{
			{ID: evalID, Force: true},
		}, gomock.Any()).Return([]*models.MatchingEvaluationSummary{
			{
				ID:           &evalID,
				ResumeStatus: &resumeStatus,
			},
		}, nil)

		req := httptest.NewRequest(http.MethodPost, "/v1/matching-evaluations/bulk-generate-resume", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusAccepted, rec.Code)
	})

	t.Run("download resume returns 401 when unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/matching-evaluations/"+evalID.String()+"/resume/download", nil)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("download resume returns 200 with download url", func(t *testing.T) {
		mockMatching.EXPECT().DownloadResume(gomock.Any(), evalID, gomock.Any()).Return(&models.DownloadResumeResponse{
			DownloadURL: "https://r2.example.com/123/resume.pdf",
			ExpiresAt:   time.Now().Add(15 * time.Minute),
			Filename:    "Alex_Resume.pdf",
		}, nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/matching-evaluations/"+evalID.String()+"/resume/download", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "https://r2.example.com/123/resume.pdf")
		assert.Contains(t, rec.Body.String(), "Alex_Resume.pdf")
	})
}

func TestUpdateEvaluation_responseKeysUnchanged(t *testing.T) {
	apiKey := "test-api-key"
	evalID := uuid.New()
	userJobID := uuid.New()
	score := 85.0
	eligibility := "eligible"
	recommendation := "strong_match"
	failMsg := "some failure"
	reportStatus := "success"
	reportFailMsg := "report failure"
	resumeStatus := "success"
	resumeFailMsg := "resume failure"
	pdfKey := "resumes/pdf.pdf"
	jsonKey := "resumes/json.json"
	now := time.Now()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMatching := usecaseMocks.NewMockMatching(ctrl)
	mockUsers := userMocks.NewMockUser(ctrl)
	rt := setupTestRouter(ctrl, mockMatching, mockUsers, apiKey)

	returnedEval := &models.MatchingEvaluation{
		ID:                   evalID,
		UserJobID:            userJobID,
		EvaluationStatus:     "success",
		EvaluationData:       []byte(`{"scores":{"technical":85}}`),
		OverallScore:         &score,
		EligibilityStatus:    &eligibility,
		RecommendationLabel:  &recommendation,
		FailureMessage:       &failMsg,
		ReportData:           []byte(`{"summary":"good"}`),
		ReportStatus:         &reportStatus,
		ReportFailureMessage: &reportFailMsg,
		ResumeStatus:         &resumeStatus,
		ResumeFailureMessage: &resumeFailMsg,
		ResumePDFKey:         &pdfKey,
		ResumeJSONKey:        &jsonKey,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	mockMatching.EXPECT().UpdateEvaluation(gomock.Any(), evalID, gomock.Any()).Return(returnedEval, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"evaluation_status": "success",
	})
	req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()

	rt.Mux().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp httpx.Response[map[string]interface{}]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	keys := resp.Data
	expectedKeys := []string{
		"id", "user_job_id", "evaluation_status", "evaluation_data", "overall_score",
		"eligibility_status", "recommendation_label", "failure_message", "report_data",
		"report_status", "report_failure_message", "resume_status", "resume_failure_message",
		"resume_pdf_key", "resume_json_key", "created_at", "updated_at",
	}
	for _, k := range expectedKeys {
		assert.Contains(t, keys, k)
	}
	assert.Nil(t, keys["job"])
}

func TestBulkGenerateCoverLetter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMatching := usecaseMocks.NewMockMatching(ctrl)
	mockUsers := userMocks.NewMockUser(ctrl)
	apiKey := "test-worker-key"
	rt := setupTestRouter(ctrl, mockMatching, mockUsers, apiKey)
	evalID := uuid.New()
	path := "/v1/matching-evaluations/bulk-generate-cover-letter"
	body := fmt.Sprintf(`[{"id":"%s","variant":"full","force":true}]`, evalID.String())

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("valid body returns 202 with queued summaries", func(t *testing.T) {
		mockMatching.EXPECT().BulkQueueCoverLetter(gomock.Any(), []models.BulkGenerateCoverLetterItem{
			{ID: evalID, Variant: "full", Force: true},
		}, gomock.Any()).Return([]*models.MatchingEvaluationSummary{{
			ID:                &evalID,
			CoverLetterStatus: stringPtr("queued_retry"),
		}}, nil)

		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)
		require.Equal(t, http.StatusAccepted, rec.Code)

		var resp httpx.Response[[]models.MatchingEvaluationSummary]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp.Data, 1)
		require.NotNil(t, resp.Data[0].ID)
		assert.Equal(t, evalID, *resp.Data[0].ID)
		require.NotNil(t, resp.Data[0].CoverLetterStatus)
		assert.Equal(t, "queued_retry", *resp.Data[0].CoverLetterStatus)
	})

	t.Run("keeps per-item errors from the usecase", func(t *testing.T) {
		errMsg := "evaluation must be successful"
		mockMatching.EXPECT().BulkQueueCoverLetter(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*models.MatchingEvaluationSummary{{ID: &evalID, Error: &errMsg}}, nil)

		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)
		require.Equal(t, http.StatusAccepted, rec.Code)
		assert.Contains(t, rec.Body.String(), errMsg)
	})

	t.Run("empty array returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`[]`))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid variant returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`[{"id":"`+evalID.String()+`","variant":"medium"}]`))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing id returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`[{"variant":"short"}]`))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestUpdateCoverLetter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMatching := usecaseMocks.NewMockMatching(ctrl)
	mockUsers := userMocks.NewMockUser(ctrl)
	apiKey := "test-worker-key"
	rt := setupTestRouter(ctrl, mockMatching, mockUsers, apiKey)
	evalID := uuid.New()
	path := "/v1/matching-evaluations/" + evalID.String() + "/cover-letter"

	t.Run("missing api key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(`{"cover_letter_status":"failed"}`))
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("success without content is rejected with 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(`{"cover_letter_status":"success"}`))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "cover_letter_content is required")
	})

	t.Run("failed status stores only the fixed message", func(t *testing.T) {
		rawException := "ValueError: leaked CV contents for Jane Doe"
		var captured usecaseMatching.UpdateEvaluationCoverLetterRequest
		mockMatching.EXPECT().UpdateEvaluationCoverLetter(gomock.Any(), evalID, gomock.Any()).DoAndReturn(
			func(_ context.Context, id uuid.UUID, req usecaseMatching.UpdateEvaluationCoverLetterRequest) (*models.MatchingEvaluation, error) {
				captured = req
				return &models.MatchingEvaluation{
					ID:                        id,
					CoverLetterStatus:         stringPtr("failed"),
					CoverLetterFailureMessage: stringPtr("Cover letter generation failed"),
				}, nil
			},
		)

		body, err := json.Marshal(map[string]interface{}{
			"cover_letter_status":          "failed",
			"cover_letter_failure_message": rawException,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		rt.Mux().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "failed", captured.CoverLetterStatus)
		require.NotNil(t, captured.CoverLetterFailureMessage)
		assert.Equal(t, rawException, *captured.CoverLetterFailureMessage)

		var resp httpx.Response[dto.MatchingEvaluationResponse]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.NotNil(t, resp.Data.CoverLetterFailureMessage)
		assert.Equal(t, "Cover letter generation failed", *resp.Data.CoverLetterFailureMessage)
		assert.NotContains(t, rec.Body.String(), "CV contents")
	})
}

func TestRetryReportHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMatching := usecaseMocks.NewMockMatching(ctrl)
	mockUsers := userMocks.NewMockUser(ctrl)
	apiKey := "test-worker-key"

	rt := setupTestRouter(ctrl, mockMatching, mockUsers, apiKey)
	evalID := uuid.New()
	path := "/v1/matching-evaluations/" + evalID.String() + "/retry-report"
	queueRetryStatus := string(constants.ReportStatusQueuedRetry)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/matching-evaluations/not-a-uuid/retry-report", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("forces report regeneration and returns 202", func(t *testing.T) {
		mockMatching.EXPECT().BulkQueueGenerateReport(gomock.Any(), []models.BulkGenerateReportItem{
			{ID: evalID, Force: true},
		}, gomock.Any()).Return([]*models.MatchingEvaluationSummary{{
			ID:           &evalID,
			ReportStatus: &queueRetryStatus,
		}}, nil)

		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)

		require.Equal(t, http.StatusAccepted, rec.Code)
		var resp httpx.Response[[]models.MatchingEvaluationSummary]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp.Data, 1)
		require.NotNil(t, resp.Data[0].ReportStatus)
		assert.Equal(t, string(constants.ReportStatusQueuedRetry), *resp.Data[0].ReportStatus)
	})

	t.Run("passes the clerk principal through to the usecase", func(t *testing.T) {
		clerk := &models.Principal{ID: "user-clerk-1"}
		mockMatching.EXPECT().BulkQueueGenerateReport(gomock.Any(), []models.BulkGenerateReportItem{
			{ID: evalID, Force: true},
		}, clerk).Return([]*models.MatchingEvaluationSummary{{
			ID:           &evalID,
			ReportStatus: &queueRetryStatus,
		}}, nil)

		c := &contract.Contract{
			Cfg: &config.Config{},
			Usecase: &contractUsecase.Contract{
				Matching: mockMatching,
				Users:    mockUsers,
			},
		}
		h := handlerMatching.NewHandler(c)
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.SetPathValue("id", evalID.String())
		req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), clerk))
		rec := httptest.NewRecorder()

		httpx.HandlerFunc(h.RetryReport).ServeHTTP(rec, req)

		require.Equal(t, http.StatusAccepted, rec.Code)
	})
}

func stringPtr(s string) *string { return &s }
