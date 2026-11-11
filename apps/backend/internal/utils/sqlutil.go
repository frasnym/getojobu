package utils

import (
	"database/sql"
	"time"
)

// NullString converts a sql.NullString to string, returning empty when null.
func NullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// NullStringPtr converts a sql.NullString to *string, returning nil when null.
func NullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// NullBoolPtr converts a sql.NullBool to *bool, returning nil when null.
func NullBoolPtr(nb sql.NullBool) *bool {
	if !nb.Valid {
		return nil
	}
	return &nb.Bool
}

// NullTime converts a sql.NullTime to time.Time, returning zero time when null.
func NullTime(nt sql.NullTime) time.Time {
	if nt.Valid {
		return nt.Time
	}
	return time.Time{}
}

// NullTimePtr converts a sql.NullTime to *time.Time, returning nil when null.
func NullTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

// TimeOrNil returns nil when t is the zero value so nullable timestamp columns
// are stored as NULL in SQL queries, otherwise returns t.
func TimeOrNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
