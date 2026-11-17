package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResumeFileName(t *testing.T) {
	tests := []struct {
		name     string
		candName string
		jobTitle string
		want     string
	}{
		{"both present", "Jane Doe", "Senior Go Engineer", "Jane_Doe_Resume_Senior-Go-Engineer.pdf"},
		{"name only", "Jane Doe", "", "Jane_Doe_Resume.pdf"},
		{"title only", "", "Senior Go Engineer", "Resume_Senior-Go-Engineer.pdf"},
		{"neither present", "", "", "Resume.pdf"},
		{"empty job title", "Jane Doe", "   ", "Jane_Doe_Resume.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resumeFileName(tt.candName, tt.jobTitle))
		})
	}
}
