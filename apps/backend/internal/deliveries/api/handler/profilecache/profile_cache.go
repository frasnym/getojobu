package profilecache

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

const maxBatch = 100

type bulkRefreshProfileRequest struct {
	UserIDs []string `json:"user_ids"`
}

// Validate parses and bounds the batch of user IDs.
func (r bulkRefreshProfileRequest) Validate() ([]uuid.UUID, error) {
	if len(r.UserIDs) == 0 {
		return nil, httpx.Envelope(httpx.CodeInvalidRequest, "user_ids must be a non-empty array", "")
	}
	if len(r.UserIDs) > maxBatch {
		return nil, httpx.Envelope(httpx.CodeInvalidRequest, fmt.Sprintf("batch size must not exceed %d items", maxBatch), "")
	}
	ids := make([]uuid.UUID, 0, len(r.UserIDs))
	for i, raw := range r.UserIDs {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, httpx.Envelope(httpx.CodeInvalidRequest, fmt.Sprintf("user_ids[%d] is required", i), "")
		}
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, httpx.Envelope(httpx.CodeInvalidRequest, fmt.Sprintf("user_ids[%d] must be a valid UUID", i), "")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type handler struct {
	cfg        *config.Config
	userUC     ucUsers.User
	matchingUC ucMatching.Matching
}

// NewHandler constructs the profile-cache handler from the contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:        c.Cfg,
		userUC:     c.Usecase.Users,
		matchingUC: c.Usecase.Matching,
	}
}

// Route registers the profile cache routes under the given router.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)
	g.POST("/bulk-refresh-profile", h.BulkRefreshProfile, auth.ValidAPIKey)
}

// BulkRefreshProfile recomposes and stores profile_data and facts_data for
// each user. Intended for machine callers after profile updates.
// @Summary Bulk refresh user profile cache
// @Description Recompose and cache candidate profile and facts for the given users. Accessible only via machine API key.
// @Tags users
// @Accept json
// @Produce json
// @Param payload body bulkRefreshProfileRequest true "User IDs to refresh"
// @Success 200 {array} models.RefreshProfileResult
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/users/bulk-refresh-profile [post]
func (h *handler) BulkRefreshProfile(w http.ResponseWriter, r *http.Request) error {
	var req bulkRefreshProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid request body", "")
	}
	ids, err := req.Validate()
	if err != nil {
		return err
	}
	results, err := h.matchingUC.RefreshProfileCache(r.Context(), ids)
	if err != nil {
		return err
	}
	return httpx.RespondJSON(w, r, http.StatusOK, results)
}
