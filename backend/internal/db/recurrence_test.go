package db

import (
	"context"
	"errors"
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
	all, err := s.ListByRecurrenceOrigin(context.Background(), originID, SystemScope)
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

	// Day 2: the modified instance carries forward and BECOMES that day's
	// occurrence — one instance per day, no duplicate fresh copy.
	if err := s.GenerateForDate(ctx, "2026-06-02"); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, "2026-06-01")); got != 0 {
		t.Fatalf("modified instance should have moved off 06-01, got %d", got)
	}
	today := instancesOn(t, s, origin, "2026-06-02")
	if len(today) != 1 {
		t.Fatalf("expected 1 instance on 06-02 (carried-forward, no duplicate), got %d", len(today))
	}
	if !today[0].IsCustomized {
		t.Fatalf("the surviving 06-02 instance should be the carried-forward customised one")
	}
}

// Reproduces the duplicate-recurring bug: a day ends up with both a customised
// instance and a bare pristine one. Generation should heal it down to one,
// keeping the customised instance.
func TestSameDayDuplicatesHealed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)
	const date = "2026-06-01"

	for i := 0; i < 2; i++ {
		if _, err := s.Create(ctx, CreateTaskParams{
			ID:                 uuid.New().String(),
			Title:              "Meditate",
			Status:             "planned",
			PlannedDate:        strptr(date),
			WeekStart:          strptr(date),
			RecurrenceOriginID: strptr(origin),
		}); err != nil {
			t.Fatalf("seed instance: %v", err)
		}
	}
	insts := instancesOn(t, s, origin, date)
	if len(insts) != 2 {
		t.Fatalf("setup expected 2 instances, got %d", len(insts))
	}
	insts[0].IsCustomized = true
	insts[0].Description = strptr("breathing focus")
	if _, err := s.Update(ctx, insts[0]); err != nil {
		t.Fatal(err)
	}

	if err := s.GenerateForDate(ctx, date); err != nil {
		t.Fatal(err)
	}
	got := instancesOn(t, s, origin, date)
	if len(got) != 1 {
		t.Fatalf("expected 1 instance after heal, got %d", len(got))
	}
	if !got[0].IsCustomized {
		t.Fatalf("heal should keep the customised instance, not the pristine one")
	}
}

// Reproduces the cross-week duplicate bug: a race (poller vs. concurrent client
// week-fetches) seeds two pristine instances on a FUTURE day. Per-day rollover
// only ever deduped "today", so the pair used to survive across the week
// boundary until that day became today. Seeding the horizon must now heal any
// duplicate on every day it touches, future days included.
func TestFutureDayDuplicatesHealedAcrossWeek(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)

	const today = "2026-06-01"     // Monday
	const futureDay = "2026-06-10" // Wednesday of the NEXT week

	// Simulate the lost race: two pristine instances land on the same future day
	// (both passed the non-atomic existence check before either inserted).
	for i := 0; i < 2; i++ {
		if _, err := s.Create(ctx, CreateTaskParams{
			ID:                 uuid.New().String(),
			Title:              "Meditate",
			Status:             "planned",
			PlannedDate:        strptr(futureDay),
			WeekStart:          strptr("2026-06-08"),
			RecurrenceOriginID: strptr(origin),
		}); err != nil {
			t.Fatalf("seed instance: %v", err)
		}
	}
	if got := len(instancesOn(t, s, origin, futureDay)); got != 2 {
		t.Fatalf("setup expected 2 instances on %s, got %d", futureDay, got)
	}

	// The poller path (timezone-agnostic, non-destructive) must heal it.
	if err := s.SeedHorizon(ctx, today, recurrenceHorizonWeeks); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, futureDay)); got != 1 {
		t.Fatalf("expected future-day duplicate healed to 1, got %d", got)
	}

	// And the client list path must be idempotent on that week too.
	if err := s.GenerateForWeek(ctx, "2026-06-08", today); err != nil {
		t.Fatal(err)
	}
	if got := len(instancesOn(t, s, origin, futureDay)); got != 1 {
		t.Fatalf("expected still 1 instance on %s after week-generate, got %d", futureDay, got)
	}
}

// Two identical pristine DONE copies of a day (the exact-same-created_at pair a
// pre-fix seed race left in history, double-counting a completed day) must
// collapse to one — but a completed instance is NEVER dropped in favour of a
// merely-planned sibling.
func TestDoneDuplicatesHealed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)
	const date = "2026-06-16"

	mkDone := func() Task {
		inst, err := s.Create(ctx, CreateTaskParams{
			ID:                 uuid.New().String(),
			Title:              "Meditate",
			Status:             "planned",
			PlannedDate:        strptr(date),
			WeekStart:          strptr("2026-06-15"),
			RecurrenceOriginID: strptr(origin),
		})
		if err != nil {
			t.Fatalf("seed instance: %v", err)
		}
		inst.Status = "done"
		done, err := s.Update(ctx, inst)
		if err != nil {
			t.Fatalf("complete instance: %v", err)
		}
		return done
	}
	mkDone()
	mkDone()
	if got := len(instancesOn(t, s, origin, date)); got != 2 {
		t.Fatalf("setup expected 2 done instances, got %d", got)
	}

	if err := s.dedupeRecurringInstances(ctx, date); err != nil {
		t.Fatal(err)
	}
	got := instancesOn(t, s, origin, date)
	if len(got) != 1 {
		t.Fatalf("expected pristine done duplicate collapsed to 1, got %d", len(got))
	}
	if got[0].Status != "done" {
		t.Fatalf("survivor must stay done, got %q", got[0].Status)
	}
}

// A completed pristine instance must outrank an open pristine sibling on the same
// day: dedup drops the planned copy and keeps the done one, never the reverse.
func TestDedupeKeepsDoneOverPlanned(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)
	const date = "2026-06-16"

	doneInst, err := s.Create(ctx, CreateTaskParams{
		ID: uuid.New().String(), Title: "Meditate", Status: "planned",
		PlannedDate: strptr(date), WeekStart: strptr("2026-06-15"),
		RecurrenceOriginID: strptr(origin),
	})
	if err != nil {
		t.Fatal(err)
	}
	doneInst.Status = "done"
	if _, err := s.Update(ctx, doneInst); err != nil {
		t.Fatal(err)
	}
	// A stray pristine planned copy on the same day.
	if _, err := s.Create(ctx, CreateTaskParams{
		ID: uuid.New().String(), Title: "Meditate", Status: "planned",
		PlannedDate: strptr(date), WeekStart: strptr("2026-06-15"),
		RecurrenceOriginID: strptr(origin),
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.dedupeRecurringInstances(ctx, date); err != nil {
		t.Fatal(err)
	}
	got := instancesOn(t, s, origin, date)
	if len(got) != 1 {
		t.Fatalf("expected 1 instance after heal, got %d", len(got))
	}
	if got[0].ID != doneInst.ID || got[0].Status != "done" {
		t.Fatalf("heal must keep the DONE instance, kept %+v", got[0])
	}
}

// Editing a template (e.g. renaming it) propagates to future untouched
// instances via SyncTemplateInstances.
func TestTemplateEditPropagates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)
	const date = "2026-06-01"

	if err := s.GenerateForDate(ctx, date); err != nil {
		t.Fatal(err)
	}
	if got := instancesOn(t, s, origin, date); len(got) != 1 || got[0].Title != "Meditate" {
		t.Fatalf("setup: expected one 'Meditate' instance, got %+v", got)
	}

	// Rename the template.
	tmpls, err := s.ListRecurringTemplates(ctx, SystemScope)
	if err != nil || len(tmpls) != 1 {
		t.Fatalf("templates: %v (%d)", err, len(tmpls))
	}
	tmpl := tmpls[0]
	tmpl.Title = "Meditate 10 min"
	if _, err := s.Update(ctx, tmpl); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncTemplateInstances(ctx, origin, date); err != nil {
		t.Fatal(err)
	}

	got := instancesOn(t, s, origin, date)
	if len(got) != 1 {
		t.Fatalf("expected 1 instance after propagate, got %d", len(got))
	}
	if got[0].Title != "Meditate 10 min" {
		t.Fatalf("instance title should reflect the edited template, got %q", got[0].Title)
	}
}

// When recurrence deletes a duplicate instance it must record a sync tombstone,
// or offline/local-first clients never drop it (the phantom-duplicate bug).
func TestRecurrenceDeleteRecordsTombstone(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)
	const date = "2026-06-01"

	ids := []string{}
	for i := 0; i < 2; i++ {
		id := uuid.New().String()
		if _, err := s.Create(ctx, CreateTaskParams{
			ID:                 id,
			Title:              "Meditate",
			Status:             "planned",
			PlannedDate:        strptr(date),
			WeekStart:          strptr(date),
			RecurrenceOriginID: strptr(origin),
		}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	// Customise the first so the dedup deletes the second (pristine) one.
	for _, inst := range instancesOn(t, s, origin, date) {
		if inst.ID == ids[0] {
			inst.IsCustomized = true
			if _, err := s.Update(ctx, inst); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := s.GenerateForDate(ctx, date); err != nil {
		t.Fatal(err)
	}

	var cnt int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM sync_tombstones WHERE entity_type='task' AND entity_id=?`,
		ids[1]).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("expected a tombstone for the deleted duplicate %s, found %d", ids[1], cnt)
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

	got, err := s.ListByWeek(ctx, weekStart, SystemScope)
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

// countOrphanedInstances returns how many tasks look like recurring instances
// that lost their template link — no rule of their own, no origin, but dated.
// This is the exact corruption a hard DELETE of a template used to cause via the
// `recurrence_origin_id ... ON DELETE SET NULL` foreign key.
func countOrphanedInstances(t *testing.T, s *TaskStore, title string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM tasks
		 WHERE title = ? AND recurrence_rule IS NULL AND recurrence_origin_id IS NULL
		   AND planned_date IS NOT NULL`, title).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestHardDeletingTemplateOrphansInstances pins the failure mode this whole
// design exists to prevent. It asserts the FK really does detach instances, so if
// anyone ever "simplifies" RetireTemplate back into a DELETE, the reason is on
// the record rather than rediscovered in production.
func TestHardDeletingTemplateOrphansInstances(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)
	if err := s.SeedHorizon(ctx, "2026-06-01", 1); err != nil {
		t.Fatal(err)
	}
	if countOrphanedInstances(t, s, "Meditate") != 0 {
		t.Fatal("precondition: instances should start out linked to their template")
	}

	if err := s.Delete(ctx, origin); err != nil {
		t.Fatal(err)
	}
	if got := countOrphanedInstances(t, s, "Meditate"); got == 0 {
		t.Fatal("expected the ON DELETE SET NULL FK to orphan instances — if this " +
			"now passes, the schema changed and RetireTemplate's rationale needs revisiting")
	}
}

func TestRetireTemplateStopsGenerationWithoutOrphaning(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)
	const today = "2026-06-01"

	if err := s.SeedHorizon(ctx, today, 1); err != nil {
		t.Fatal(err)
	}
	seeded, err := s.ListByRecurrenceOrigin(ctx, origin, SystemScope)
	if err != nil {
		t.Fatal(err)
	}
	if len(seeded) == 0 {
		t.Fatal("expected the horizon to seed instances")
	}

	// Mark one instance done so it counts as history that must be preserved.
	done := seeded[0]
	done.Status = "done"
	if _, err := s.Update(ctx, done); err != nil {
		t.Fatal(err)
	}

	if err := s.RetireTemplate(ctx, origin); err != nil {
		t.Fatal(err)
	}

	// 1. Nothing was orphaned — the whole point.
	if got := countOrphanedInstances(t, s, "Meditate"); got != 0 {
		t.Fatalf("retire orphaned %d instances; expected 0", got)
	}

	// 2. The done instance survives, still linked to its template.
	kept, err := s.Get(ctx, done.ID, SystemScope)
	if err != nil {
		t.Fatalf("done instance should survive retirement: %v", err)
	}
	if kept.RecurrenceOriginID == nil || *kept.RecurrenceOriginID != origin {
		t.Fatalf("done instance lost its template link: %v", kept.RecurrenceOriginID)
	}

	// 3. Open pristine instances are gone, and tombstoned so offline clients drop them.
	for _, inst := range seeded {
		if inst.ID == done.ID {
			continue
		}
		if _, err := s.Get(ctx, inst.ID, SystemScope); !errors.Is(err, ErrNotFound) {
			t.Fatalf("pristine instance %s should have been removed, got %v", inst.ID, err)
		}
		var n int
		if err := s.db.QueryRowContext(ctx,
			`SELECT count(*) FROM sync_tombstones WHERE entity_type='task' AND entity_id=?`,
			inst.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("expected a tombstone for removed instance %s, found %d", inst.ID, n)
		}
	}

	// 4. The template is hidden from the generators and from Settings → Recurring.
	tmpls, err := s.ListRecurringTemplates(ctx, SystemScope)
	if err != nil {
		t.Fatal(err)
	}
	for _, tm := range tmpls {
		if tm.ID == origin {
			t.Fatal("a retired template must not be listed as a template")
		}
	}

	// 5. Generation really has stopped — a later horizon seeds nothing new.
	if err := s.SeedHorizon(ctx, "2026-06-08", 1); err != nil {
		t.Fatal(err)
	}
	after, err := s.ListByRecurrenceOrigin(ctx, origin, SystemScope)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("retired template still generating: expected only the kept done instance, got %d", len(after))
	}
}

func TestRetiredTemplateHiddenFromSearch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)

	found, err := s.Search(ctx, "meditate", nil, false, 50, SystemScope)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("precondition: an active template should be findable")
	}

	if err := s.RetireTemplate(ctx, origin); err != nil {
		t.Fatal(err)
	}
	found, err = s.Search(ctx, "meditate", nil, false, 50, SystemScope)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range found {
		if task.ID == origin {
			t.Fatal("a retired template must not surface in search")
		}
	}
}

func TestRetireTemplateRejectsNonTemplates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	plain, err := s.Create(ctx, CreateTaskParams{
		ID: uuid.New().String(), Title: "Not recurring", Status: "backlog",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RetireTemplate(ctx, plain.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound retiring a non-template, got %v", err)
	}
}
