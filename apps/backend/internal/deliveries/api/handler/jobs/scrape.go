package jobs

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// ScrapeJobs accepts a scrape request and starts JobScrapeWorkflow in Temporal
// for each normalized search target derived from user preferences.
//
// @Summary Queue a job scrape scan
// @Description Start job discovery scans for active user preferences. Returns 202 with the workflow ids. Machine callers only (API key auth).
// @Tags jobs
// @Accept json
// @Produce json
// @Param request body models.ScrapeJobsRequest false "Optional user filter"
// @Success 202 {object} models.ScrapeJobsResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/scrape [post]
func (h *handler) ScrapeJobs(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var req models.ScrapeJobsRequest
	if r.Body != nil {
		buf, err := io.ReadAll(r.Body)
		if err == nil && len(strings.TrimSpace(string(buf))) > 0 {
			r.Body = io.NopCloser(bytes.NewReader(buf))
			if err := httpx.ParseJSON(r, &req); err != nil {
				return err
			}
		}
	}

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var cleanedUserIDs []string
	for _, uid := range req.UserIDs {
		if s := strings.TrimSpace(uid); s != "" {
			cleanedUserIDs = append(cleanedUserIDs, s)
		}
	}

	workflowIDs, err := h.ucJobs.StartJobScrape(ctx, models.ScrapeJobsParams{
		UserIDs: cleanedUserIDs,
	})
	if err != nil {
		logging.Error(ctx, "start job scrape workflows failed", err,
			"actor_type", p.ActorType, "actor_id", p.ActorID,
			"user_ids_count", len(cleanedUserIDs))
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusAccepted, models.ScrapeJobsResponse{
		WorkflowIDs: workflowIDs,
	})
}
