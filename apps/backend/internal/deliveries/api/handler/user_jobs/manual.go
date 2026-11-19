package user_jobs

import (
	"net/http"
	"strings"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/dto"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// createManualJobRequest is the POST /v1/user-jobs/manual body.
type createManualJobRequest struct {
	Title          string  `json:"title"`
	Company        string  `json:"company"`
	Location       string  `json:"location"`
	Description    *string `json:"description,omitempty"`
	SourceURL      *string `json:"source_url,omitempty"`
	ApplyURL       *string `json:"apply_url,omitempty"`
	PostedDate     string  `json:"posted_date,omitempty"`
	IsRemote       *bool   `json:"is_remote,omitempty"`
	VisaRelocation *bool   `json:"visa_relocation,omitempty"`
}

// CreateManualJob inserts a private job owned by the authenticated user. The
// job is linked to the user like any other saved job and enters the normal
// analysis pipeline.
// @Summary Create manual job
// @Description Create a private job from user-supplied fields. Only the owning Clerk user can see it.
// @Tags user-jobs
// @Accept json
// @Produce json
// @Param job body createManualJobRequest true "Manual job data"
// @Success 201 {object} dto.CreateJobResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/user-jobs/manual [post]
func (h *handler) CreateManualJob(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req createManualJobRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Company) == "" || strings.TrimSpace(req.Location) == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "title, company, and location are required", "")
	}

	var postedDate time.Time
	if req.PostedDate != "" {
		parsed, err := time.Parse("2006-01-02", req.PostedDate)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, req.PostedDate)
		}
		if err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, "posted_date must be in YYYY-MM-DD or RFC3339 format", "")
		}
		postedDate = parsed
	}

	result, err := h.ucJobs.CreateManual(r.Context(), p.ID, models.CreateManualJobParams{
		Title:          req.Title,
		Company:        req.Company,
		Location:       req.Location,
		Description:    req.Description,
		SourceURL:      req.SourceURL,
		ApplyURL:       req.ApplyURL,
		PostedDate:     postedDate,
		IsRemote:       req.IsRemote,
		VisaRelocation: req.VisaRelocation,
	})
	if err != nil {
		logging.Error(r.Context(), "create manual job failed", err, "user_id", p.ID)
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusCreated, dto.NewCreateJobResponse(result))
}
