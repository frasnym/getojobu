package utils

// PtrString returns a pointer to the given string, or nil if empty.
func PtrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
