package db

import (
	"testing"
	"time"
)

// isDueOn parses a stored recurrence-rule string ("daily", "weekly:1,3",
// "monthly:31", …). Fuzz it across a range of dates to ensure no rule string
// can panic the recurrence generator.
func FuzzIsDueOn(f *testing.F) {
	for _, s := range []string{
		"daily", "weekdays", "weekends",
		"weekly:1,3,5", "weekly:", "weekly:99", "weekly:abc",
		"monthly:15", "monthly:31", "monthly:0", "monthly:abc",
		"", "unknown",
	} {
		f.Add(s, int64(0))
	}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, rule string, offset int64) {
		// Keep dates in a sane window; we only care that no input panics.
		d := base.AddDate(0, 0, int(offset%4000))
		_ = isDueOn(rule, d)
	})
}
