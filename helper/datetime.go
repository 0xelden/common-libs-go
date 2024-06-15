package helper

import (
	"time"

	"github.com/araddon/dateparse"
)

// CompareYYMMDD compare time.Time to a precision of a calendar date
func CompareYYMMDD(a, b time.Time) int {
	// Truncate the time components to 0 (midnight)
	truncatedA := a.Truncate(24 * time.Hour)
	truncatedB := b.Truncate(24 * time.Hour)

	// Compare the truncated dates
	if truncatedA.Before(truncatedB) {
		return -1
	} else if truncatedA.After(truncatedB) {
		return 1
	}
	return 0
}

func ParseDate(date string, layout ...string) time.Time {
	for _, v := range layout {
		if v, err := time.Parse(v, date); err == nil {
			return v
		}
	}
	return First(dateparse.ParseAny(date))
}

// FirstLastOfMonth returns the first nanosecond of the month
// and the last nanosecond **of the same month** in the local zone.
func FirstLastOfMonth(t time.Time) (time.Time, time.Time) {
	loc := t.Location()
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)

	// Move to 1 ⧸ next month, then step back 1 ns → last instant of this month
	last := first.AddDate(0, 1, 0).Add(-time.Nanosecond)

	return first, last
}
