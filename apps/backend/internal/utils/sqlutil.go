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

// NullTime converts a sql.NullTime to time.Time, returning zero time when null.
func NullTime(nt sql.NullTime) time.Time {
	if nt.Valid {
		return nt.Time
	}
	return time.Time{}
}
