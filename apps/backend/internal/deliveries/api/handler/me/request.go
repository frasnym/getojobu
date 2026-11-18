package me

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// editableProfileKeys is the exact set of fields PATCH /v1/me accepts. A
// request must include all of them; null clears the corresponding field.
var editableProfileKeys = []string{
	"full_name",
	"headline",
	"phone",
	"portfolio_url",
	"linkedin_url",
	"github_url",
}

// decodeUpdateMeRequest validates the self-editable account fields. It rejects
// unknown keys (including role and email), requires all six keys to be present,
// trims non-null strings, and enforces per-field limits.
func decodeUpdateMeRequest(r *http.Request) (models.UserProfileInput, *httpx.Error) {
	var input models.UserProfileInput
	invalid := func() *httpx.Error {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid account details", "")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return models.UserProfileInput{}, invalid()
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(body, &keys); err != nil {
		return models.UserProfileInput{}, invalid()
	}

	errResp := invalid()
	for _, key := range editableProfileKeys {
		if _, ok := keys[key]; !ok {
			errResp.AddField(key, "field is required")
		}
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		return models.UserProfileInput{}, errResp
	}

	validateText := func(key string, value *string, max int, required bool) {
		if value == nil {
			return
		}
		trimmed := strings.TrimSpace(*value)
		*value = trimmed
		if required && trimmed == "" {
			errResp.AddField(key, "must not be blank")
			return
		}
		if utf8.RuneCountInString(trimmed) > max {
			errResp.AddField(key, "is too long")
		}
	}

	validateText("full_name", input.FullName, 128, true)
	validateText("headline", input.Headline, 256, true)
	validateText("phone", input.Phone, 64, false)

	validateURL := func(key string, value **string) {
		if *value == nil {
			return
		}
		trimmed := strings.TrimSpace(**value)
		**value = trimmed
		if utf8.RuneCountInString(trimmed) > 2048 {
			errResp.AddField(key, "is too long")
			return
		}
		u, err := url.Parse(trimmed)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			errResp.AddField(key, "must be an http or https URL with a host")
		}
	}

	validateURL("portfolio_url", &input.PortfolioURL)
	validateURL("linkedin_url", &input.LinkedInURL)
	validateURL("github_url", &input.GitHubURL)

	if len(errResp.Details) > 0 {
		return models.UserProfileInput{}, errResp
	}
	return input, nil
}
