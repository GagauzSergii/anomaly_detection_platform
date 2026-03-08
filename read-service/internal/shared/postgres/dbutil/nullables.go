package dbutil

import "github.com/jackc/pgx/v5/pgtype"

// TextPtr converts *string to pgtype.Text (NULL-safe).
func TextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// Int8Ptr converts *int64 to pgtype.Int8 (NULL-safe).
func Int8Ptr(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
