package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFileNamePart(t *testing.T) {
	assert.Equal(t, "Jane Doe", SanitizeFileNamePart("  Jane Doe  "))
	assert.Equal(t, "Jane Doe", SanitizeFileNamePart("Jane/ Doe\\"))
	assert.Equal(t, "abc", SanitizeFileNamePart("a:b*c?\"<>|"))
	assert.Equal(t, "abcd", SanitizeFileNamePart("a\nb\rc\td"))
	assert.Empty(t, SanitizeFileNamePart(string(rune(0))))
}
