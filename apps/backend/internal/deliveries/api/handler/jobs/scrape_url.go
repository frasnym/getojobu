package jobs

import (
	"net/http"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/dto"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// scrapeURLRequest is the POST /v1/jobs/scrape-url body.
type scrapeURLRequest struct {
	SourceURL string `json:"source_url"`
}

// ScrapeURL creates a private skeleton job from a URL and queues an LLM scrape.
// The actual job fields are filled in by UserJobScrapeWorkflow.
// @Summary Scrape job from URL
// @Description Create a private job from a URL. The job starts at scrape_pending and is filled in by the scrape workflow.
// @Tags jobs
// @Accept json
// @Produce json
// @Param job body scrapeURLRequest true "Job URL"
// @Success 202 {object} dto.CreateJobResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/scrape-url [post]
func (h *handler) ScrapeURL(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req scrapeURLRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if !strings.HasPrefix(req.SourceURL, "http://") && !strings.HasPrefix(req.SourceURL, "https://") {
		return httpx.Envelope(httpx.CodeInvalidRequest, "source_url must be an http(s) URL", "")
	}

	result, err := h.ucJobs.ScrapeURL(r.Context(), p.ID, models.ScrapeURLParams{SourceURL: req.SourceURL})
	if err != nil {
		logging.Error(r.Context(), "scrape job from URL failed", err, "user_id", p.ID)
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusAccepted, dto.NewCreateJobResponse(result))
}
