package jobs

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs/mocks"
	userMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/users/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestScrapeJobs_unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := `{"user_ids":["userA"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ScrapeJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestScrapeJobs_success_accepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		StartJobScrape(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx any, params models.ScrapeJobsParams) ([]string, error) {
			assert.Equal(t, []string{"userA"}, params.UserIDs)
			return []string{"job-scrape-glassdoor-golang-ireland"}, nil
		})

	body := `{"user_ids":["userA"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ScrapeJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp httpx.Response[models.ScrapeJobsResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, []string{"job-scrape-glassdoor-golang-ireland"}, resp.Data.WorkflowIDs)
}

func TestScrapeJobs_cleans_user_ids(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		StartJobScrape(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx any, params models.ScrapeJobsParams) ([]string, error) {
			assert.Equal(t, []string{"userA", "userB"}, params.UserIDs)
			return []string{"job-scrape-glassdoor-golang-ireland"}, nil
		})

	body := `{"user_ids":["  userA ", "", "   ", "userB"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ScrapeJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestScrapeJobs_empty_body_defaults_all(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		StartJobScrape(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx any, params models.ScrapeJobsParams) ([]string, error) {
			assert.Empty(t, params.UserIDs)
			return []string{"job-scrape-glassdoor-golang-indonesia", "job-scrape-linkedin-golang-indonesia"}, nil
		})

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ScrapeJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp httpx.Response[models.ScrapeJobsResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data.WorkflowIDs, 2)
}

func TestScrapeJobs_usecase_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		StartJobScrape(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("temporal failure"))

	body := `{"user_ids":["userA"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ScrapeJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestScrapeJobs_route_machine_auth(t *testing.T) {
	cfg := &config.Config{
		MachineAuth: config.MachineAuth{
			APIKeys: map[string]config.MachineKey{
				"valid-cron-key": {ActorType: "cron", ActorID: "cron-scrape"},
			},
		},
	}

	t.Run("authorized", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockJob := ucMocks.NewMockJob(ctrl)
		mockUser := userMocks.NewMockUser(ctrl)
		mockJob.EXPECT().
			StartJobScrape(gomock.Any(), gomock.Any()).
			Return([]string{"job-scrape-scan-valid"}, nil)

		h := &handler{cfg: cfg, userUC: mockUser, ucJobs: mockJob}
		router := httpx.New()
		h.Route(router.Group("/v1/jobs"))

		body := `{"user_ids":["userA"]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-cron-key")
		w := httptest.NewRecorder()

		router.Mux().ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		var resp httpx.Response[models.ScrapeJobsResponse]
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.True(t, resp.Success)
		assert.Equal(t, []string{"job-scrape-scan-valid"}, resp.Data.WorkflowIDs)
	})

	t.Run("missing_auth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockJob := ucMocks.NewMockJob(ctrl)
		mockUser := userMocks.NewMockUser(ctrl)
		h := &handler{cfg: cfg, userUC: mockUser, ucJobs: mockJob}
		router := httpx.New()
		h.Route(router.Group("/v1/jobs"))

		body := `{"user_ids":["userA"]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.Mux().ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid_key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockJob := ucMocks.NewMockJob(ctrl)
		mockUser := userMocks.NewMockUser(ctrl)
		h := &handler{cfg: cfg, userUC: mockUser, ucJobs: mockJob}
		router := httpx.New()
		h.Route(router.Group("/v1/jobs"))

		body := `{"user_ids":["userA"]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer bad-key")
		w := httptest.NewRecorder()

		router.Mux().ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
