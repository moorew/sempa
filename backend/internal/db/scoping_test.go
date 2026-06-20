package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// newScopingFixture spins up a DB with two users — A (primary) and B — plus the
// stores under test, for the per-user isolation checks below.
func newScopingFixture(t *testing.T) (*TaskStore, *ListStore, *ObjectiveStore, *DailyPlanStore, *SyncStore, string, string) {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "scope.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := NewUserStore(conn)
	ctx := context.Background()
	a, err := users.Create(ctx, CreateUserParams{Email: "a@example.com", IsAdmin: true})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := users.Create(ctx, CreateUserParams{Email: "b@example.com"})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	return NewTaskStore(conn), NewListStore(conn), NewObjectiveStore(conn),
		NewDailyPlanStore(conn), NewSyncStore(conn), a.ID, b.ID
}

func mustCreateTask(t *testing.T, s *TaskStore, owner string, shared bool, date string) Task {
	t.Helper()
	task, err := s.Create(context.Background(), CreateTaskParams{
		ID: uuid.New().String(), Title: "t", Status: "planned", PlannedDate: &date, OwnerID: owner, Shared: shared,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

func ids(tasks []Task) map[string]bool {
	m := map[string]bool{}
	for _, t := range tasks {
		m[t.ID] = true
	}
	return m
}

func TestScoping_TasksPrivateVsShared(t *testing.T) {
	ctx := context.Background()
	tasks, _, _, _, _, userA, userB := newScopingFixture(t)
	const date = "2026-06-20"

	aPriv := mustCreateTask(t, tasks, userA, false, date)
	aShared := mustCreateTask(t, tasks, userA, true, date)
	bPriv := mustCreateTask(t, tasks, userB, false, date)

	// A sees their private + their shared, never B's private.
	got, err := tasks.ListByDate(ctx, date, userA)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	seen := ids(got)
	if !seen[aPriv.ID] || !seen[aShared.ID] {
		t.Errorf("A should see own tasks: %v", seen)
	}
	if seen[bPriv.ID] {
		t.Errorf("LEAK: A can see B's private task")
	}

	// B sees their own + A's SHARED, never A's private.
	got, _ = tasks.ListByDate(ctx, date, userB)
	seen = ids(got)
	if !seen[bPriv.ID] {
		t.Errorf("B should see own task")
	}
	if !seen[aShared.ID] {
		t.Errorf("B should see A's shared task")
	}
	if seen[aPriv.ID] {
		t.Errorf("LEAK: B can see A's private task")
	}

	// Direct Get is scoped too.
	if _, err := tasks.Get(ctx, aPriv.ID, userB); err != ErrNotFound {
		t.Errorf("LEAK: B Get of A's private task returned %v, want ErrNotFound", err)
	}
	if _, err := tasks.Get(ctx, aShared.ID, userB); err != nil {
		t.Errorf("B should Get A's shared task, got %v", err)
	}
}

func TestScoping_SyncPullExcludesOthersPrivate(t *testing.T) {
	ctx := context.Background()
	tasks, _, _, _, sync, userA, userB := newScopingFixture(t)
	const date = "2026-06-20"
	aPriv := mustCreateTask(t, tasks, userA, false, date)
	aShared := mustCreateTask(t, tasks, userA, true, date)
	bPriv := mustCreateTask(t, tasks, userB, false, date)

	ch, err := sync.Changes(ctx, "", userB)
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	seen := ids(ch.Tasks)
	if seen[aPriv.ID] {
		t.Errorf("LEAK: B's sync pulled A's private task")
	}
	if !seen[aShared.ID] {
		t.Errorf("B's sync should include A's shared task")
	}
	if !seen[bPriv.ID] {
		t.Errorf("B's sync should include own task")
	}
}

func TestScoping_PersonalPlansNeverShared(t *testing.T) {
	ctx := context.Background()
	_, _, _, plans, _, userA, userB := newScopingFixture(t)
	const date = "2026-06-20"
	intention := "ship multi-user"
	if _, err := plans.Upsert(ctx, UpsertPlanParams{
		ID: uuid.New().String(), PlanDate: date, Status: "active", Intention: &intention, OwnerID: userA,
	}); err != nil {
		t.Fatalf("upsert A plan: %v", err)
	}
	// B must not see A's plan (plans are personal, never shared).
	if _, err := plans.Get(ctx, date, userB); err != ErrNotFound {
		t.Errorf("LEAK: B saw A's daily plan, got %v", err)
	}
	if _, err := plans.Get(ctx, date, userA); err != nil {
		t.Errorf("A should see own plan, got %v", err)
	}
	// Both users can have a plan for the SAME date (composite unique).
	if _, err := plans.Upsert(ctx, UpsertPlanParams{
		ID: uuid.New().String(), PlanDate: date, Status: "active", Intention: &intention, OwnerID: userB,
	}); err != nil {
		t.Errorf("B should be able to plan the same date as A: %v", err)
	}
}

func TestScoping_UnshareRevokesFromPeerNotOwner(t *testing.T) {
	ctx := context.Background()
	_, _, _, _, sync, userA, userB := newScopingFixture(t)

	// A un-shares an entity: record a revocation owned by A.
	if err := sync.RecordRevocation(ctx, "task", "task-1", userA); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Peer B is told to drop it…
	chB, _ := sync.Changes(ctx, "", userB)
	if !hasDeletion(chB.Deletions, "task-1") {
		t.Errorf("peer B should receive the revocation for task-1")
	}
	// …but owner A is NOT (they still hold the now-private row).
	chA, _ := sync.Changes(ctx, "", userA)
	if hasDeletion(chA.Deletions, "task-1") {
		t.Errorf("owner A must NOT receive a revocation for their own task")
	}
}

func TestScoping_DeleteTombstoneRoutedToViewers(t *testing.T) {
	ctx := context.Background()
	_, _, _, _, sync, userA, userB := newScopingFixture(t)

	// A deletes a PRIVATE task → only A's other devices should drop it.
	if err := sync.RecordTombstone(ctx, "task", "priv-1", userA, false); err != nil {
		t.Fatalf("tombstone: %v", err)
	}
	// A deletes a SHARED task → everyone should drop it.
	if err := sync.RecordTombstone(ctx, "task", "shared-1", userA, true); err != nil {
		t.Fatalf("tombstone: %v", err)
	}

	chB, _ := sync.Changes(ctx, "", userB)
	if hasDeletion(chB.Deletions, "priv-1") {
		t.Errorf("LEAK: B received deletion of A's private task")
	}
	if !hasDeletion(chB.Deletions, "shared-1") {
		t.Errorf("B should receive deletion of the shared task")
	}
	chA, _ := sync.Changes(ctx, "", userA)
	if !hasDeletion(chA.Deletions, "priv-1") {
		t.Errorf("A should receive deletion of their own task")
	}
}

func hasDeletion(ds []Tombstone, id string) bool {
	for _, d := range ds {
		if d.EntityID == id {
			return true
		}
	}
	return false
}
