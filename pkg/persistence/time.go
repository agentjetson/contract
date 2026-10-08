package persistence

import "time"

// TruncateMS truncates t to millisecond precision in UTC,
// matching DateTime64(3, 'UTC') columns in seed/sql/002_tables.sql.
func TruncateMS(t time.Time) time.Time {
	return t.UTC().Truncate(time.Millisecond)
}

// NowMS is TruncateMS(time.Now()).
func NowMS() time.Time {
	return TruncateMS(time.Now())
}
