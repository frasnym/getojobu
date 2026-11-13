package matching_test

import (
	"bytes"
	"encoding/json"
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

func TestAnalyzeCompatibilityHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMatching := usecaseMocks.NewMockMatching(ctrl)
	mockUsers := userMocks.NewMockUser(ctrl)
	apiKey := "test-worker-key"

	rt := setupTestRouter(ctrl, mockMatching, mockUsers, apiKey)

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/test-slug/analyze-compatibility", nil)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("worker with api key triggers compatibility successfully", func(t *testing.T) {
		mockMatching.EXPECT().TriggerCompatibility(gomock.Any(), "test-slug", gomock.Any()).Return([]*models.MatchingEvaluationSummary{
			{
				ID:     uuid.New(),
				Status: string(constants.MatchingStatusNew),
			},
		}, nil)

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/test-slug/analyze-compatibility", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusAccepted, rec.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		mockMatching.EXPECT().TriggerCompatibility(gomock.Any(), "unknown-slug", gomock.Any()).Return(nil, constants.ErrNotFound)

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/unknown-slug/analyze-compatibility", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()

		rt.Mux().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
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
}
