package db

import (
	"strings"
	"sync/atomic"
	"time"
)

// Server timezone: the single authoritative location for every server-side
// "today" / day-boundary calculation (recurrence rollover, the recurrence
// horizon poller, the morning digest). The container's wall clock is almost
// always UTC, so computing "today" with a bare time.Now() rolls the date over
// at the wrong moment for the user — e.g. midnight UTC is 8pm US-Eastern, which
// is exactly how a pristine "today" recurring instance got deleted hours early.
//
// Tasks themselves are *floating* (naive YYYY-MM-DD + HH:MM, never rewritten on
// travel), so the only thing the zone governs is when the day turns over. Active
// clients drive exact rollover by passing their own device date; this location
// is the fallback for code paths with no client (the poller, the digest) and the
// home anchor a travelling user is compared against.
//
// Resolution order, applied at boot and on every settings change:
//
//	notifications.timezone setting  >  SEMPA_TIMEZONE env  >  UTC
//
// serverLoc holds the active value; fallbackLoc holds the env default so that
// clearing the setting falls back to the env rather than to raw UTC.
var (
	serverLoc   atomic.Pointer[time.Location]
	fallbackLoc atomic.Pointer[time.Location]
)

// SetFallbackLocation records the env-derived default (SEMPA_TIMEZONE). Call once
// at boot, before SetServerLocation.
func SetFallbackLocation(loc *time.Location) {
	if loc == nil {
		loc = time.UTC
	}
	fallbackLoc.Store(loc)
}

func fallback() *time.Location {
	if l := fallbackLoc.Load(); l != nil {
		return l
	}
	return time.UTC
}

// ResolveLocation parses an IANA timezone name (e.g. "America/Toronto"). An empty
// or unparseable name resolves to the env fallback, never to a hard UTC, so a
// blank setting still honours SEMPA_TIMEZONE.
func ResolveLocation(name string) *time.Location {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback()
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return fallback()
	}
	return loc
}

// SetServerLocation sets the active server timezone. A nil location falls back to
// the env default.
func SetServerLocation(loc *time.Location) {
	if loc == nil {
		loc = fallback()
	}
	serverLoc.Store(loc)
}

// ServerLocation returns the active server timezone (env fallback if unset).
func ServerLocation() *time.Location {
	if l := serverLoc.Load(); l != nil {
		return l
	}
	return fallback()
}

// ServerNow returns the current time in the configured server timezone.
func ServerNow() time.Time { return time.Now().In(ServerLocation()) }

// ServerToday returns today's date (YYYY-MM-DD) in the configured server
// timezone — the correct fallback wherever a client's device date is absent.
func ServerToday() string { return ServerNow().Format("2006-01-02") }
