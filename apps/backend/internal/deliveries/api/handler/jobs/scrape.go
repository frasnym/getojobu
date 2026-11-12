package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// searchHash computes a deterministic id from the sorted keywords and
// locations, used to de-duplicate scans for the same owner + search set.
func searchHash(src models.ScrapeSearch) string {
	ks := slices.Clone(src.Keywords)
	ls := slices.Clone(src.Locations)
	sort.Strings(ks)
	sort.Strings(ls)
	raw := strings.Join(ks, "|") + "-" + strings.Join(ls, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:12]
}

// ScrapeJobs accepts a scrape request, validates it, and starts the
// JobScrapeWorkflow in Temporal.
//
// @Summary Queue a job scrape scan
// @Description Start a job discovery scan for a given set of keyword × location searches. Returns 202 with the workflow id. Machine callers only (API key auth).
// @Tags jobs
// @Accept json
// @Produce json
// @Param request body models.ScrapeJobsRequest true "Search queries"
// @Success 202 {object} models.ScrapeJobsResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/scrape [post]
func (h *handler) ScrapeJobs(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var req models.ScrapeJobsRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if len(req.Searches) == 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "at least one search is required", "")
	}

	for _, src := range req.Searches {
		if len(src.Keywords) == 0 {
			return httpx.Envelope(httpx.CodeInvalidRequest, "each search must have at least one keyword", "")
		}
		if len(src.Locations) == 0 {
			return httpx.Envelope(httpx.CodeInvalidRequest, "each search must have at least one location", "")
		}
	}

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	// Stable workflow id per owner + search set (deterministic across runs).
	// A-mode: job-scrape-scan-{hash}. B-mode: job-scrape-{user_id}-{hash}.
	hd := searchHash(req.Searches[0])
	workflowID := "job-scrape-scan-" + hd
	if p.ActorID != "" && p.ActorID != "cron-scrape" {
		workflowID = "job-scrape-" + p.ActorID + "-" + hd
	}

	out, err := h.ucJobs.StartJobScrape(ctx, models.ScrapeJobsParams{
		UserID:     p.ID,
		SearchID:   hd,
		Searches:   req.Searches,
		WorkflowID: workflowID,
	})
	if err != nil {
		logging.Error(ctx, "start job scrape workflow failed", err,
			"actor_type", p.ActorType, "actor_id", p.ActorID,
			"search_count", len(req.Searches))
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusAccepted, models.ScrapeJobsResponse{
		WorkflowID: out,
	})
}
