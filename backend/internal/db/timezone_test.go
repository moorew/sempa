package db

import (
	"context"
	"testing"
	"time"
)

// TestResolveLocation covers the env-fallback resolution order.
func TestResolveLocation(t *testing.T) {
	SetFallbackLocation(time.UTC)
	if got := ResolveLocation(""); got != time.UTC {
		t.Fatalf("empty should resolve to fallback UTC, got %v", got)
	}
	if got := ResolveLocation("Definitely/NotAZone"); got != time.UTC {
		t.Fatalf("invalid should resolve to fallback UTC, got %v", got)
	}
	tor := ResolveLocation("America/Toronto")
	if tor.String() != "America/Toronto" {
		t.Fatalf("expected America/Toronto, got %v", tor)
	}

	// A non-UTC fallback is honoured when the name is empty/invalid.
	SetFallbackLocation(tor)
	if got := ResolveLocation(""); got.String() != "America/Toronto" {
		t.Fatalf("empty should fall back to env zone, got %v", got)
	}
	SetFallbackLocation(time.UTC) // restore for other tests
}

// TestSeedHorizonKeepsCurrentInstances is the regression guard for the original
// bug: the recurrence poller must NOT delete a still-current pristine instance
// at the server's own midnight. SeedHorizon only prunes instances older than a
// 2-day grace window (safely beyond any timezone spread) and seeds forward.
func TestSeedHorizonKeepsCurrentInstances(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)

	// A pristine instance for "yesterday" (well within grace) must survive a poll.
	if err := s.GenerateForDate(ctx, "2026-06-09"); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-09")); got != 1 {
		t.Fatalf("setup: expected 1 instance on 06-09, got %d", got)
	}

	if err := s.SeedHorizon(ctx, "2026-06-10", 2); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-09")); got != 1 {
		t.Fatalf("poller deleted a within-grace instance (the original bug): 06-09 had %d", got)
	}
	// And it seeds today + the horizon forward.
	if got := len(instancesOn(t, s, origin, "2026-06-10")); got != 1 {
		t.Fatalf("expected seeded instance on today 06-10, got %d", got)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-17")); got != 1 {
		t.Fatalf("expected seeded instance a week out, got %d", got)
	}
}

// TestSeedHorizonPrunesStale confirms the 2-day grace cleanup still bounds
// pile-up of long-abandoned pristine instances.
func TestSeedHorizonPrunesStale(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)

	if err := s.GenerateForDate(ctx, "2026-06-01"); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedHorizon(ctx, "2026-06-10", 2); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-01")); got != 0 {
		t.Fatalf("expected stale 06-01 instance pruned, got %d", got)
	}
}

// TestGenerateForDayAnchorsToToday guards against deleting today's instance when
// the user simply pages to a future day. Rollover must key off the client's real
// today, not the date being viewed.
func TestGenerateForDayAnchorsToToday(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)

	if err := s.GenerateForDate(ctx, "2026-06-10"); err != nil {
		t.Fatal(err)
	}

	// View a day a week ahead; today is still 06-10.
	if err := s.GenerateForDay(ctx, "2026-06-17", "2026-06-10"); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-10")); got != 1 {
		t.Fatalf("paging to a future day deleted today's instance: 06-10 had %d", got)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-17")); got != 1 {
		t.Fatalf("viewed future day not materialised: 06-17 had %d", got)
	}
}
