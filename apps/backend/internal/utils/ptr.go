package utils

// PtrString returns a pointer to the given string, or nil if empty.
func PtrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// PtrFloatOrZero returns the dereferenced float64, or 0 if nil.
func PtrFloatOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

// PtrStringOrEmpty returns the dereferenced string, or "" if nil.
func PtrStringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
