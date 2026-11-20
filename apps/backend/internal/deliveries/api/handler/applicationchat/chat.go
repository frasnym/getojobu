package applicationchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	ucApplicationChat "github.com/frasnym/getojobu/apps/backend/internal/usecase/applicationchat"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

const (
	maxQuestionChars = 2000
	maxHistoryTurns  = 10
	maxHistoryChars  = 2000
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Question string        `json:"question"`
	History  []chatMessage `json:"history"`
}

type handler struct {
	cfg     *config.Config
	chatSvc *ucApplicationChat.Service
	jobsUC  ucJobs.Job
	usersUC ucUsers.User
}

// NewHandler builds the application-chat handler from the shared contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:     c.Cfg,
		chatSvc: ucApplicationChat.NewService(c.Cfg.OmniRoute),
		jobsUC:  c.Usecase.Jobs,
		usersUC: c.Usecase.Users,
	}
}

// Route registers the chat endpoint under the jobs group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.usersUC)
	g.POST("/{slug}/chat", h.Chat, auth.ValidClerkSession)
}

// Chat answers one application-form question from the authenticated user's
// profile and the analyzed job profile. It streams plain-text SSE frames.
// @Summary Answer an application form question
// @Description Streams a one-sentence answer grounded in the caller's career profile and the analyzed job profile.
// @Tags jobs
// @Accept json
// @Produce text/event-stream
// @Param slug path string true "Job slug"
// @Param payload body chatRequest true "Question and optional in-memory history"
// @Success 200 {string} string "SSE data frames with delta, then done"
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/{slug}/chat [post]
func (h *handler) Chat(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	principal := ctxval.PrincipalFromContext(ctx)
	if principal == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req chatRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}
	if err := req.validate(); err != nil {
		return err
	}

	slug := httpx.ExtractPathParam(r, "slug")
	if slug == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "job slug is required", "")
	}

	jobWithOwnership, err := h.jobsUC.GetBySlug(ctx, slug, principal.ID)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
		}
		return err
	}
	jobProfile, err := ucMatching.DeriveJobProfile(jobWithOwnership.Job)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "job has not been analyzed", "")
	}
	user, err := h.usersUC.FindByID(ctx, principal.ID)
	if err != nil {
		return err
	}
	if user == nil {
		return httpx.Envelope(httpx.CodeNotFound, "user not found", "")
	}

	history := make([]ucApplicationChat.Message, 0, len(req.History))
	for _, turn := range req.History {
		history = append(history, ucApplicationChat.Message{Role: turn.Role, Content: turn.Content})
	}
	input := ucApplicationChat.Input{
		JobProfile: jobProfile,
		Candidate:  user,
		Question:   req.Question,
		History:    history,
	}

	streamCtx, cancel := context.WithTimeout(ctx, h.cfg.OmniRoute.Timeout)
	defer cancel()

	started := false
	controller := http.NewResponseController(w)
	emit := func(text string) error {
		if !started {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			started = true
		}
		frame, err := json.Marshal(map[string]string{"delta": text})
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", frame); err != nil {
			return err
		}
		// The server's global write timeout would otherwise cut a slow stream.
		_ = controller.SetWriteDeadline(time.Now().Add(15 * time.Second))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}

	if err := h.chatSvc.StreamAnswer(streamCtx, input, emit); err != nil {
		if !started {
			return fmt.Errorf("application chat failed: %w", err)
		}
		frame, _ := json.Marshal(map[string]string{"error": "answer generation failed"})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", frame)
		return nil
	}

	if _, err := fmt.Fprint(w, "data: {\"done\":true}\n\n"); err != nil {
		return err
	}
	return nil
}

func (r *chatRequest) validate() error {
	r.Question = strings.TrimSpace(r.Question)
	if r.Question == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "question is required", "")
	}
	if len(r.Question) > maxQuestionChars {
		return httpx.Envelope(httpx.CodeInvalidRequest,
			fmt.Sprintf("question must not exceed %d characters", maxQuestionChars), "")
	}
	if len(r.History) > maxHistoryTurns {
		return httpx.Envelope(httpx.CodeInvalidRequest,
			fmt.Sprintf("history must not exceed %d turns", maxHistoryTurns), "")
	}
	for i, turn := range r.History {
		if turn.Role != "user" && turn.Role != "assistant" {
			return httpx.Envelope(httpx.CodeInvalidRequest,
				fmt.Sprintf("history[%d].role must be user or assistant", i), "")
		}
		if len(turn.Content) > maxHistoryChars {
			return httpx.Envelope(httpx.CodeInvalidRequest,
				fmt.Sprintf("history[%d].content is too long", i), "")
		}
	}
	return nil
}
