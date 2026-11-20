package applicationchat

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

//go:embed prompt/*.txt
var prompts embed.FS

const (
	promptVersion   = "answer-application-question-v1"
	maxContextChars = 12000
)

// Message is one prior turn of the in-memory chat. Nothing is persisted.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Input carries only server-derived context plus the user's question.
type Input struct {
	JobProfile map[string]interface{}
	Candidate  *models.User
	Question   string
	History    []Message
}

// Service calls OmniRoute directly to answer one application-form question.
// It is the only Go path that performs an LLM call.
type Service struct {
	cfg    config.OmniRouteConfig
	client *http.Client
}

// NewService builds the chat service from the OmniRoute configuration.
func NewService(cfg config.OmniRouteConfig) *Service {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Service{cfg: cfg, client: &http.Client{Timeout: timeout}}
}

// StreamAnswer asks OmniRoute for a one-sentence answer and calls emit for each
// text chunk. SSE is used when the gateway returns text/event-stream; a plain
// JSON completion is accepted as a single chunk otherwise.
func (s *Service) StreamAnswer(ctx context.Context, in Input, emit func(string) error) error {
	if s.cfg.APIKey == "" {
		return errors.New("omniroute api key is not configured")
	}
	if in.Candidate == nil {
		return errors.New("candidate profile is required")
	}

	system, err := s.renderPrompt(in)
	if err != nil {
		return err
	}

	messages := make([]map[string]string, 0, len(in.History)+2)
	messages = append(messages, map[string]string{"role": "system", "content": system})
	for _, turn := range in.History {
		messages = append(messages, map[string]string{"role": turn.Role, "content": turn.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": in.Question})

	body, err := json.Marshal(map[string]interface{}{
		"model":       s.cfg.Model,
		"stream":      true,
		"temperature": 0,
		"max_tokens":  200,
		"messages":    messages,
	})
	if err != nil {
		return fmt.Errorf("marshal omniroute payload: %w", err)
	}

	url := strings.TrimRight(s.cfg.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build omniroute request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")
	// OmniRoute's persistent memory must not inject unrelated context here.
	req.Header.Set("x-omniroute-no-memory", "true")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("omniroute request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		return fmt.Errorf("omniroute returned status %d", resp.StatusCode)
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return streamSSE(resp.Body, emit)
	}
	return streamJSON(resp.Body, emit)
}

func (s *Service) renderPrompt(in Input) (string, error) {
	template, err := prompts.ReadFile("prompt/" + promptVersion + ".txt")
	if err != nil {
		return "", fmt.Errorf("read application-chat prompt: %w", err)
	}

	jobJSON, err := json.MarshalIndent(in.JobProfile, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal job profile: %w", err)
	}
	candidateJSON, err := json.MarshalIndent(candidateContext(in.Candidate), "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal candidate profile: %w", err)
	}

	rendered := strings.ReplaceAll(string(template), "{{job_profile}}", truncate(string(jobJSON), maxContextChars))
	rendered = strings.ReplaceAll(rendered, "{{candidate_profile}}", truncate(string(candidateJSON), maxContextChars))
	return rendered, nil
}

// candidateContext keeps the profile fields the LLM needs. The User struct
// marks ProfileData json:"-", so it must be added explicitly here. FactsData
// is deliberately excluded: it carries contact details not needed for form
// answers.
func candidateContext(user *models.User) map[string]interface{} {
	return map[string]interface{}{
		"full_name":     deref(user.FullName),
		"headline":      deref(user.Headline),
		"portfolio_url": deref(user.PortfolioURL),
		"linkedin_url":  deref(user.LinkedInURL),
		"github_url":    deref(user.GitHubURL),
		"profile_data":  json.RawMessage(user.ProfileData),
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n[truncated]"
}

func streamSSE(r io.Reader, emit func(string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		// Progress and metadata frames are not answer text; skip them.
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if err := emit(choice.Delta.Content); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func streamJSON(r io.Reader, emit func(string) error) error {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&response); err != nil {
		return fmt.Errorf("decode omniroute response: %w", err)
	}
	if len(response.Choices) == 0 {
		return errors.New("omniroute returned no choices")
	}
	text := response.Choices[0].Message.Content
	if text == "" {
		return errors.New("omniroute returned an empty answer")
	}
	return emit(text)
}
