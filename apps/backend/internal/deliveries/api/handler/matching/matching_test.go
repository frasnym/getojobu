package matching_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	contractUsecase "github.com/frasnym/getojobu/apps/backend/internal/contract/usecase"
	handlerJobs "github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/jobs"
	handlerMatching "github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/matching"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	usecaseMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching/mocks"
	userMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/users/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
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
				Slug:   "job-1",
				ID:     &newID,
				Status: string(constants.MatchingStatusNew),
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
			ID:     evalID,
			Status: string(constants.MatchingStatusPending),
		}, nil)

		body, err := json.Marshal(map[string]interface{}{
			"status": "pending",
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
			"status": "pending",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/matching-evaluations/"+evalID.String(), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
