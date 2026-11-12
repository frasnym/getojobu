package utils

import (
	"strings"
	"unicode"
)

// Slugify trims, lowercases, kebab-cases (replaces non-alphanumeric runs with
// a single hyphen), and strips leading/trailing hyphens.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteRune('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
