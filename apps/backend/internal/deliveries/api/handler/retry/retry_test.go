package retry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	contractUsecase "github.com/frasnym/getojobu/apps/backend/internal/contract/usecase"
	handlerRetry "github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/retry"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	retryMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/retry/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

const (
	jobsPath     = "/v1/maintenance/retry-failed-jobs"
	matchingPath = "/v1/maintenance/retry-failed-matching-evaluations"
	apiKey       = "test-worker-key"
	machineActor = "worker-1"
)

func setupRouter(ctrl *gomock.Controller) (*httpx.Router, *retryMocks.MockRetry) {
	cfg := &config.Config{
		MachineAuth: config.MachineAuth{
			APIKeys: map[string]config.MachineKey{
				apiKey: {ActorType: "worker", ActorID: machineActor},
			},
		},
	}
	mockRetry := retryMocks.NewMockRetry(ctrl)
	c := &contract.Contract{
		Cfg: cfg,
		Usecase: &contractUsecase.Contract{
			Retry: mockRetry,
		},
	}
	rt := httpx.New()
	handlerRetry.NewHandler(c).Route(rt.Group("/v1").Group("/maintenance"))
	return rt, mockRetry
}

func serve(rt *httpx.Router, path string, authorized bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if authorized {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	rec := httptest.NewRecorder()
	rt.Mux().ServeHTTP(rec, req)
	return rec
}

func TestRetryFailedJobsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rt, mockRetry := setupRouter(ctrl)

	t.Run("rejects missing api key", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, serve(rt, jobsPath, false).Code)
	})

	t.Run("returns requeued count", func(t *testing.T) {
		mockRetry.EXPECT().FailedJobs(gomock.Any(), gomock.Eq(&models.Principal{ActorType: "worker", ActorID: machineActor})).
			Return(models.RetryJobsResult{Requeued: 3}, nil)
		rec := serve(rt, jobsPath, true)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"requeued":3`)
	})

	t.Run("propagates usecase error", func(t *testing.T) {
		mockRetry.EXPECT().FailedJobs(gomock.Any(), gomock.Any()).Return(models.RetryJobsResult{}, context.DeadlineExceeded)
		assert.Equal(t, http.StatusInternalServerError, serve(rt, jobsPath, true).Code)
	})
}

func TestRetryFailedMatchingEvaluationsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rt, mockRetry := setupRouter(ctrl)

	t.Run("rejects missing api key", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, serve(rt, matchingPath, false).Code)
	})

	t.Run("returns counts per kind", func(t *testing.T) {
		mockRetry.EXPECT().FailedMatchingEvaluations(gomock.Any(), gomock.Any()).
			Return(models.RetryMatchingEvaluationsResult{Evaluations: 2, Reports: 1, Resumes: 1, CoverLetters: 1}, nil)
		rec := serve(rt, matchingPath, true)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"evaluations":2`)
		assert.Contains(t, rec.Body.String(), `"cover_letters":1`)
	})

	t.Run("propagates usecase error", func(t *testing.T) {
		mockRetry.EXPECT().FailedMatchingEvaluations(gomock.Any(), gomock.Any()).
			Return(models.RetryMatchingEvaluationsResult{}, context.DeadlineExceeded)
		assert.Equal(t, http.StatusInternalServerError, serve(rt, matchingPath, true).Code)
	})
}
