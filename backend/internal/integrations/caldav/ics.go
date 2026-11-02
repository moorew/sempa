package caldav

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// TaskUIDPrefix marks calendar events that originate from Sempa tasks, so they
// can be recognised (and filtered out of the read-side display) elsewhere.
const TaskUIDPrefix = "sempa-task-"

// TaskUID returns the deterministic CalDAV UID for a task. Using a stable UID
// makes PUT an idempotent upsert and DELETE addressable without a mapping table.
func TaskUID(taskID string) string {
	return TaskUIDPrefix + taskID
}

// IsTaskUID reports whether a VEVENT UID was created by Sempa from a task.
func IsTaskUID(uid string) bool {
	return strings.HasPrefix(uid, TaskUIDPrefix)
}

// EventInput is the data needed to render a VEVENT.
type EventInput struct {
	UID         string
	Summary     string
	Description string
	URL         string
	Start       time.Time
	End         time.Time
}

// BuildVCALENDAR renders a single-event VCALENDAR document with UTC timestamps.
func BuildVCALENDAR(in EventInput) string {
	now := time.Now().UTC().Format(icalUTC)
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Sempa//Task Calendar//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	writeLine(&b, "UID", in.UID)
	writeLine(&b, "DTSTAMP", now)
	writeLine(&b, "DTSTART", in.Start.UTC().Format(icalUTC))
	writeLine(&b, "DTEND", in.End.UTC().Format(icalUTC))
	writeLine(&b, "SUMMARY", escapeText(in.Summary))
	if in.Description != "" {
		writeLine(&b, "DESCRIPTION", escapeText(in.Description))
	}
	if in.URL != "" {
		writeLine(&b, "URL", escapeText(in.URL))
	}
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

const icalUTC = "20060102T150405Z"

// ParseTime accepts the timestamp formats stored on a task (RFC3339, with or
// without a zone) and returns a time.Time.
func ParseTime(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("caldav: unrecognised time %q", s)
}

// writeLine emits a property, folding lines longer than 75 octets (RFC 5545)
// without splitting a UTF-8 character across the fold.
func writeLine(b *strings.Builder, name, value string) {
	line := name + ":" + value
	for len(line) > 75 {
		cut := 75
		for cut > 1 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		b.WriteString(line[:cut])
		b.WriteString("\r\n ")
		line = line[cut:]
	}
	b.WriteString(line)
	b.WriteString("\r\n")
}

// escapeText escapes a value for an iCalendar TEXT property (RFC 5545 §3.3.11).
//
// Line breaks of every kind become an escaped \n, and other control characters
// (which TEXT may not contain) are dropped. A bare CR used to pass through, and
// a parser that treats CR as a line break would read the rest of the task's
// title as new properties (an ATTENDEE, say) in the calendar owner's account.
func escapeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, ";", "\\;")
	return s
}
