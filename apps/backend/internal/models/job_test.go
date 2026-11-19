package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildJobSlug(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		company  string
		location string
		want     string
	}{
		{
			name:     "standard ASCII",
			title:    "Senior Backend Engineer",
			company:  "Acme Corp",
			location: "Jakarta",
			want:     "senior-backend-engineer-acme-corp-jakarta",
		},
		{
			name:     "punctuation and special characters",
			title:    "C++ Developer / Architect (Remote!)",
			company:  "AT&T Inc.",
			location: "New York, NY",
			want:     "c-developer-architect-remote-at-t-inc-new-york-ny",
		},
		{
			name:     "all empty strings fall back to defaults",
			title:    "",
			company:  "",
			location: "",
			want:     "untitled-unknown-company-unspecified-location",
		},
		{
			name:     "spaces and hyphens trimmed",
			title:    "  Frontend---Lead  ",
			company:  " -Google- ",
			location: " Singapore ",
			want:     "frontend-lead-google-singapore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildJobSlug(tt.title, tt.company, tt.location)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildManualJobSlug(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		title    string
		company  string
		location string
		want     string
	}{
		{
			name:     "normal",
			userID:   "abcdef12-3456-7890-abcd-ef1234567890",
			title:    "Senior Engineer",
			company:  "Acme",
			location: "Berlin",
			want:     "m-abcdef12-senior-engineer-acme-berlin",
		},
		{
			name:     "different user same job",
			userID:   "12345678-3456-7890-abcd-ef1234567890",
			title:    "Senior Engineer",
			company:  "Acme",
			location: "Berlin",
			want:     "m-12345678-senior-engineer-acme-berlin",
		},
		{
			name:     "short user id",
			userID:   "abc",
			title:    "Senior Engineer",
			company:  "Acme",
			location: "Berlin",
			want:     "m-abc-senior-engineer-acme-berlin",
		},
		{
			name:     "empty fields",
			userID:   "abcdef12-3456-7890-abcd-ef1234567890",
			title:    "",
			company:  "",
			location: "",
			want:     "m-abcdef12-untitled-unknown-company-unspecified-location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, BuildManualJobSlug(tt.userID, tt.title, tt.company, tt.location))
		})
	}

	first := BuildManualJobSlug("abcdef12-3456-7890-abcd-ef1234567890", "Senior Engineer", "Acme", "Berlin")
	second := BuildManualJobSlug("12345678-3456-7890-abcd-ef1234567890", "Senior Engineer", "Acme", "Berlin")
	assert.NotEqual(t, first, second)
}
