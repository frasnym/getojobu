package applicationchat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

func testInput() Input {
	return Input{
		JobProfile: map[string]interface{}{
			"title":        "Backend Engineer",
			"seniority":    "senior",
			"requirements": []interface{}{"Go"},
		},
		Candidate: &models.User{
			FullName:    strPtr("Jane Doe"),
			ProfileData: json.RawMessage(`{"years":5}`),
			FactsData:   json.RawMessage(`{"email":"jane@example.com"}`),
		},
		Question: "How many years of Go experience do you have?",
	}
}

func strPtr(value string) *string { return &value }

func TestAnswer(t *testing.T) {
	var gotPayload map[string]interface{}
	var gotHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-omniroute-no-memory")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotPayload))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Five years of Go."}}]}`))
	}))
	defer server.Close()

	svc := NewService(config.OmniRouteConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "use-combo",
		Timeout: 5 * time.Second,
	})

	answer, err := svc.Answer(context.Background(), testInput())

	require.NoError(t, err)
	assert.Equal(t, "Five years of Go.", answer)
	assert.Equal(t, "true", gotHeader)
	assert.Equal(t, "use-combo", gotPayload["model"])
	assert.Equal(t, false, gotPayload["stream"])
	assert.Equal(t, float64(200), gotPayload["max_tokens"])

	messages := gotPayload["messages"].([]interface{})
	system := messages[0].(map[string]interface{})["content"].(string)
	assert.Contains(t, system, "Backend Engineer")
	assert.Contains(t, system, "Jane Doe")
	assert.Contains(t, system, `"years": 5`)
	assert.NotContains(t, system, "jane@example.com")
	assert.NotContains(t, system, "facts_data")
	assert.NotContains(t, system, "{{job_profile}}")
	assert.Equal(t, testInput().Question, messages[len(messages)-1].(map[string]interface{})["content"])
}

func TestAnswer_UpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream unavailable"))
	}))
	defer server.Close()

	svc := NewService(config.OmniRouteConfig{BaseURL: server.URL, APIKey: "k", Model: "m", Timeout: time.Second})
	_, err := svc.Answer(context.Background(), testInput())
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "502"))
}
