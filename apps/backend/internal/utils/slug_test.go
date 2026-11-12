package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal text", "Software Engineer", "software-engineer"},
		{"symbols and spaces", "Senior Go / Python Developer (Remote)!", "senior-go-python-developer-remote"},
		{"multiple dashes and whitespace", "  ---hello---world---  ", "hello-world"},
		{"empty string", "", ""},
		{"unicode and digits", "Go 1.27 Release", "go-1-27-release"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Slugify(tt.input))
		})
	}
}
