package ical

import (
	"strings"
	"testing"
)

// Parse consumes untrusted calendar feeds (any URL the user subscribes to), so
// it must never panic on malformed input. Fuzz it; failures are persisted to
// testdata/fuzz for regression.
func FuzzParse(f *testing.F) {
	f.Add("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nSUMMARY:Hi\r\nDTSTART:20260101T120000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	f.Add("BEGIN:VEVENT\nSUMMARY:no end")
	f.Add("DTSTART;TZID=America/Toronto:20260101T090000")
	f.Add("")
	f.Fuzz(func(t *testing.T, in string) {
		_, _ = Parse(strings.NewReader(in))
	})
}
