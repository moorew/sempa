package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// mkDone inserts a completed task with the given estimate/actual (0 = unset).
func mkDone(t *testing.T, s *TaskStore, title string, est, act int64, tags []string) {
	t.Helper()
	p := CreateTaskParams{
		ID:     uuid.New().String(),
		Title:  title,
		Status: "done",
		Tags:   tags,
	}
	if est > 0 {
		p.TimeEstimateMinutes = &est
	}
	created, err := s.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("create %q: %v", title, err)
	}
	if act > 0 {
		created.TimeActualMinutes = &act
	}
	created.Status = "done"
	if _, err := s.Update(context.Background(), created); err != nil {
		t.Fatalf("update %q: %v", title, err)
	}
}

// The whole reason CompletedDurationSamples exists: duration learning must see
// tasks that were completed with logged time but never estimated. The multiplier
// query requires an estimate (a ratio needs a denominator) and would silently
// drop them, shrinking the pool and biasing it toward carefully-planned work.
func TestDurationSamplesIncludeUnestimatedTasks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	mkDone(t, s, "Reply to Avery", 15, 12, []string{"work"}) // estimated + logged
	mkDone(t, s, "Clear inbox", 0, 20, []string{"work"})     // logged, never estimated
	mkDone(t, s, "Plan the week", 30, 0, nil)                // estimated, no logged time

	durations, err := s.CompletedDurationSamples(ctx, 100, SystemScope)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, d := range durations {
		got[d.Title] = d.Minutes
	}
	if len(durations) != 2 {
		t.Fatalf("expected 2 duration samples, got %d (%v)", len(durations), got)
	}
	if got["Clear inbox"] != 20 {
		t.Errorf("unestimated task must be included; got %v", got)
	}
	if _, ok := got["Plan the week"]; ok {
		t.Errorf("a task with no logged time is not a duration sample; got %v", got)
	}

	// Contrast: the multiplier query keeps requiring both numbers.
	ratios, err := s.CompletedTimeSamples(ctx, 100, SystemScope)
	if err != nil {
		t.Fatal(err)
	}
	if len(ratios) != 1 || ratios[0].Title != "Reply to Avery" {
		t.Fatalf("CompletedTimeSamples should only return estimated+logged tasks, got %+v", ratios)
	}
}

// Insights are a personal calibration, so another user's completed work must
// never leak into your profile — even when that work is shared.
func TestDurationSamplesAreOwnerOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	mine := uuid.New().String()
	theirs := uuid.New().String()

	for _, owner := range []string{mine, theirs} {
		task, err := s.Create(ctx, CreateTaskParams{
			ID: uuid.New().String(), Title: "Shared chore", Status: "done",
			OwnerID: owner, Shared: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		mins := int64(25)
		task.TimeActualMinutes = &mins
		task.Status = "done"
		if _, err := s.Update(ctx, task); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.CompletedDurationSamples(ctx, 100, mine)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only my own sample, got %d — shared work must not calibrate my profile", len(got))
	}
}
