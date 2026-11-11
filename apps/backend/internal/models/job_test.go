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
