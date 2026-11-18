package utils

import "strings"

// SanitizeFileNamePart strips characters that are unsafe in file names and
// trims surrounding whitespace. It is used for the optional name parts of the
// on-demand cover letter download filename.
func SanitizeFileNamePart(value string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\n', '\r', '\t', 0:
			return -1
		default:
			return r
		}
	}, value))
}
