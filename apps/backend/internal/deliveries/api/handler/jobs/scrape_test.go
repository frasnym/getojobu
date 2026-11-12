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

func TestScrapeJobs_validation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		wantStatus int
		wantCode   httpx.Code
	}{
		{"empty searches", `{"searches":[]}`, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"missing keyword", `{"searches":[{"locations":["ireland"]}]}`, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"missing location", `{"searches":[{"keywords":["golang"]}]}`, http.StatusBadRequest, httpx.CodeInvalidRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			h, _, _ := newHandlerWithMock(ctrl)

			req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
			w := httptest.NewRecorder()

			httpx.HandlerFunc(h.ScrapeJobs).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			var resp httpx.Response[any]
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.False(t, resp.Success)
			assert.Equal(t, tc.wantCode, resp.Error.Code)
		})
	}
}

func TestScrapeJobs_unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := `{"searches":[{"keywords":["golang"],"locations":["ireland"]}]}`
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
		DoAndReturn(func(ctx any, params models.ScrapeJobsParams) (string, error) {
			assert.Equal(t, "cron-scrape", params.UserID)
			assert.NotEmpty(t, params.SearchID)
			assert.True(t, strings.HasPrefix(params.WorkflowID, "job-scrape-scan-"))
			return "job-scrape-scan-a1b2c3", nil
		})

	body := `{"searches":[{"keywords":["golang"],"locations":["ireland"]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("cron", "cron-scrape")))
	w := httptest.NewRecorder()

	httpx.HandlerFunc(h.ScrapeJobs).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp httpx.Response[models.ScrapeJobsResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "job-scrape-scan-a1b2c3", resp.Data.WorkflowID)
}

func TestScrapeJobs_usecase_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockJob, _ := newHandlerWithMock(ctrl)

	mockJob.EXPECT().
		StartJobScrape(gomock.Any(), gomock.Any()).
		Return("", errors.New("temporal failure"))

	body := `{"searches":[{"keywords":["golang"],"locations":["ireland"]}]}`
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
			Return("job-scrape-scan-valid", nil)

		h := &handler{cfg: cfg, userUC: mockUser, ucJobs: mockJob}
		router := httpx.New()
		h.Route(router.Group("/v1/jobs"))

		body := `{"searches":[{"keywords":["golang"],"locations":["ireland"]}]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-cron-key")
		w := httptest.NewRecorder()

		router.Mux().ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		var resp httpx.Response[models.ScrapeJobsResponse]
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.True(t, resp.Success)
		assert.Equal(t, "job-scrape-scan-valid", resp.Data.WorkflowID)
	})

	t.Run("missing_auth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockJob := ucMocks.NewMockJob(ctrl)
		mockUser := userMocks.NewMockUser(ctrl)
		h := &handler{cfg: cfg, userUC: mockUser, ucJobs: mockJob}
		router := httpx.New()
		h.Route(router.Group("/v1/jobs"))

		body := `{"searches":[{"keywords":["golang"],"locations":["ireland"]}]}`
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

		body := `{"searches":[{"keywords":["golang"],"locations":["ireland"]}]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/scrape", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer bad-key")
		w := httptest.NewRecorder()

		router.Mux().ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestScrapeJobs_hash_deterministic(t *testing.T) {
	s1 := models.ScrapeSearch{Keywords: []string{"golang", "backend"}, Locations: []string{"ireland"}}
	s2 := models.ScrapeSearch{Keywords: []string{"backend", "golang"}, Locations: []string{"ireland"}}
	h1 := searchHash(s1)
	h2 := searchHash(s2)
	assert.Equal(t, h1, h2, "same set, different order -> same hash")
	assert.Len(t, h1, 12, "hash should be 12 hex characters")

	s3 := models.ScrapeSearch{Keywords: []string{"golang"}, Locations: []string{"ireland"}}
	assert.NotEqual(t, h1, searchHash(s3), "different set -> different hash")
}
