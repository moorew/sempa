package caldav

import (
	"strings"
	"testing"
	"time"
)

// Task text must never add content lines to the PUT body: a bare CR used to
// survive, and a CR-splitting parser would read it as a new property.
func TestBuildVCALENDARNoInjectedLines(t *testing.T) {
	start := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	out := BuildVCALENDAR(EventInput{
		UID:         "u1",
		Summary:     "x\rATTENDEE:mailto:b@example.test\r\nORGANIZER:mailto:o@example.test\x07",
		Description: "line1\nline2\r",
		Start:       start,
		End:         start.Add(time.Hour),
	})
	for _, line := range strings.Split(out, "\r\n") {
		if strings.ContainsAny(line, "\r\n\x07") {
			t.Fatalf("control character left inside a content line: %q", line)
		}
		if strings.HasPrefix(line, "ATTENDEE") || strings.HasPrefix(line, "ORGANIZER") {
			t.Fatalf("task text became a property: %q", line)
		}
	}
}

func TestWriteLineFoldsOnRuneBoundary(t *testing.T) {
	var b strings.Builder
	writeLine(&b, "SUMMARY", strings.Repeat("é", 60))
	for _, part := range strings.Split(strings.TrimSuffix(b.String(), "\r\n"), "\r\n ") {
		if !utf8ValidString(part) {
			t.Fatalf("fold split a UTF-8 character: %q", part)
		}
	}
}

func utf8ValidString(s string) bool { return strings.ToValidUTF8(s, "�") == s }
