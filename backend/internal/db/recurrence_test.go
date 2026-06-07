package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *TaskStore {
	t.Helper()
	dbConn, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Migrate(dbConn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { dbConn.Close() })
	return NewTaskStore(dbConn)
}

func strptr(s string) *string { return &s }

// makeDailyTemplate inserts a recurring daily template and returns its id.
func makeDailyTemplate(t *testing.T, s *TaskStore, roughlyAt *string) string {
	t.Helper()
	tmpl, err := s.Create(context.Background(), CreateTaskParams{
		ID:             uuid.New().String(),
		Title:          "Meditate",
		Status:         "backlog",
		RecurrenceRule: strptr("daily"),
		RoughlyAt:      roughlyAt,
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	return tmpl.ID
}

func instancesOn(t *testing.T, s *TaskStore, originID, date string) []Task {
	t.Helper()
	all, err := s.ListByRecurrenceOrigin(context.Background(), originID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var out []Task
	for _, task := range all {
		if task.PlannedDate != nil && *task.PlannedDate == date && task.Status != "cancelled" {
			out = append(out, task)
		}
	}
	return out
}

func TestPristineRolloverReplaces(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)

	// Day 1: generate yesterday's instance.
	if err := s.GenerateForDate(ctx, "2026-06-01"); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-01")); got != 1 {
		t.Fatalf("expected 1 instance on 06-01, got %d", got)
	}

	// Day 2: pristine yesterday instance should be deleted, today gets a fresh one.
	if err := s.GenerateForDate(ctx, "2026-06-02"); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-01")); got != 0 {
		t.Fatalf("expected pristine 06-01 instance deleted, got %d", got)
	}
	today := instancesOn(t, s, origin, "2026-06-02")
	if len(today) != 1 {
		t.Fatalf("expected 1 instance on 06-02, got %d", len(today))
	}
	// week_start must match the Monday of 2026-06-02 (a Tuesday → 2026-06-01).
	if today[0].WeekStart == nil || *today[0].WeekStart != "2026-06-01" {
		t.Fatalf("expected week_start 2026-06-01, got %v", today[0].WeekStart)
	}
}

func TestModifiedRolloverCarriesForwardPlusNew(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)

	if err := s.GenerateForDate(ctx, "2026-06-01"); err != nil {
		t.Fatal(err)
	}
	inst := instancesOn(t, s, origin, "2026-06-01")[0]

	// User modifies the instance (adds a note) → is_customized.
	inst.Description = strptr("breathing focus")
	inst.IsCustomized = true
	if _, err := s.Update(ctx, inst); err != nil {
		t.Fatal(err)
	}

	// Day 2: modified instance carries forward AND a fresh one is created.
	if err := s.GenerateForDate(ctx, "2026-06-02"); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-01")); got != 0 {
		t.Fatalf("modified instance should have moved off 06-01, got %d", got)
	}
	today := instancesOn(t, s, origin, "2026-06-02")
	if len(today) != 2 {
		t.Fatalf("expected 2 instances on 06-02 (carried + new), got %d", len(today))
	}
}

func TestWeekGenerationFindsInstanceByWeekStart(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, strptr("07:30"))

	// Home view path: generate today's instance.
	if err := s.GenerateForDate(ctx, "2026-06-03"); err != nil { // Wednesday
		t.Fatal(err)
	}
	// Day/week view path: generate for the week using the client's today.
	weekStart := "2026-06-01" // Monday
	if err := s.GenerateForWeek(ctx, weekStart, "2026-06-03"); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListByWeek(ctx, weekStart)
	if err != nil {
		t.Fatal(err)
	}
	var found *Task
	for i := range got {
		if got[i].RecurrenceOriginID != nil && *got[i].RecurrenceOriginID == origin &&
			got[i].PlannedDate != nil && *got[i].PlannedDate == "2026-06-03" {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatal("today's recurring instance not found via ListByWeek (week_start mismatch)")
	}
	if found.RoughlyAt == nil || *found.RoughlyAt != "07:30" {
		t.Fatalf("expected roughly_at 07:30 copied to instance, got %v", found.RoughlyAt)
	}
}
