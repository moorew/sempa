package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func newProjectFixture(t *testing.T) (*sql.DB, *ProjectStore, *TaskStore, string, string) {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "projects.db"))
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
	return conn, NewProjectStore(conn), NewTaskStore(conn), a.ID, b.ID
}

func TestProjects_SeededStatusesAndDefault(t *testing.T) {
	ctx := context.Background()
	_, ps, _, userA, _ := newProjectFixture(t)

	sts, err := ps.Statuses(ctx)
	if err != nil {
		t.Fatalf("statuses: %v", err)
	}
	if len(sts) != 5 {
		t.Fatalf("want 5 seeded statuses, got %d", len(sts))
	}
	p, err := ps.Create(ctx, CreateProjectParams{ID: uuid.New().String(), Title: "Garden", OwnerID: userA})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.StatusID != "active" {
		t.Fatalf("default status = %q, want active", p.StatusID)
	}
}

func TestProjects_DeleteUnlinksAndBumps(t *testing.T) {
	ctx := context.Background()
	conn, ps, tasks, userA, _ := newProjectFixture(t)

	p, err := ps.Create(ctx, CreateProjectParams{ID: uuid.New().String(), Title: "Kitchen", OwnerID: userA})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	task, err := tasks.Create(ctx, CreateTaskParams{
		ID: uuid.New().String(), Title: "tiles", Status: "backlog", OwnerID: userA, ProjectID: &p.ID,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	// Age the row so the bump is observable at 1s resolution.
	if _, err := conn.Exec(`UPDATE tasks SET updated_at = '2000-01-01 00:00:00' WHERE id = ?`, task.ID); err != nil {
		t.Fatal(err)
	}

	links, err := ps.Delete(ctx, p.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if links.Tasks != 1 {
		t.Fatalf("unlinked tasks = %d, want 1", links.Tasks)
	}
	got, err := tasks.Get(ctx, task.ID, userA)
	if err != nil {
		t.Fatalf("task survived? %v", err)
	}
	if got.ProjectID != nil {
		t.Fatalf("project_id = %v, want nil", *got.ProjectID)
	}
	if got.UpdatedAt == "2000-01-01 00:00:00" {
		t.Fatal("unlink did not bump updated_at, so offline clients would never see it")
	}
}

func TestProjects_DeleteStatusReassigns(t *testing.T) {
	ctx := context.Background()
	_, ps, _, userA, _ := newProjectFixture(t)

	extra, err := ps.CreateStatus(ctx, ProjectStatus{ID: uuid.New().String(), Name: "Blocked", Color: "#000", Category: "on_hold"})
	if err != nil {
		t.Fatalf("create status: %v", err)
	}
	p, err := ps.Create(ctx, CreateProjectParams{ID: uuid.New().String(), Title: "x", StatusID: extra.ID, OwnerID: userA})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	repl, err := ps.DeleteStatus(ctx, extra.ID)
	if err != nil {
		t.Fatalf("delete status: %v", err)
	}
	if repl != "on_hold" {
		t.Fatalf("replacement = %q, want the same-category status on_hold", repl)
	}
	got, _ := ps.Get(ctx, p.ID, userA)
	if got.StatusID != "on_hold" {
		t.Fatalf("project status = %q, want on_hold", got.StatusID)
	}

	// Deleting down to one status works; deleting the last is refused.
	sts, _ := ps.Statuses(ctx)
	for _, st := range sts[1:] {
		if _, err := ps.DeleteStatus(ctx, st.ID); err != nil {
			t.Fatalf("delete %s: %v", st.ID, err)
		}
	}
	if _, err := ps.DeleteStatus(ctx, sts[0].ID); err != ErrLastStatus {
		t.Fatalf("deleting last status: err = %v, want ErrLastStatus", err)
	}
}

func TestProjects_ScopingAndSync(t *testing.T) {
	ctx := context.Background()
	conn, ps, _, userA, userB := newProjectFixture(t)
	sync := NewSyncStore(conn)

	priv, _ := ps.Create(ctx, CreateProjectParams{ID: uuid.New().String(), Title: "private", OwnerID: userA})
	shared, _ := ps.Create(ctx, CreateProjectParams{ID: uuid.New().String(), Title: "shared", OwnerID: userA, Shared: true})

	bList, err := ps.List(ctx, userB)
	if err != nil {
		t.Fatal(err)
	}
	if len(bList) != 1 || bList[0].ID != shared.ID {
		t.Fatalf("B sees %v, want only the shared project", bList)
	}
	if _, err := ps.Get(ctx, priv.ID, userB); err != ErrNotFound {
		t.Fatalf("B Get(private) err = %v, want ErrNotFound", err)
	}

	ch, err := sync.Changes(ctx, "", userB)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Projects) != 1 || ch.Projects[0].ID != shared.ID {
		t.Fatalf("B sync projects = %v, want only shared", ch.Projects)
	}
	if len(ch.ProjectStatuses) != 5 {
		t.Fatalf("sync statuses = %d, want 5 (global)", len(ch.ProjectStatuses))
	}
}

func TestProjects_DescendantsFollow(t *testing.T) {
	ctx := context.Background()
	_, ps, tasks, userA, _ := newProjectFixture(t)

	p, _ := ps.Create(ctx, CreateProjectParams{ID: uuid.New().String(), Title: "p", OwnerID: userA})
	parent, _ := tasks.Create(ctx, CreateTaskParams{ID: uuid.New().String(), Title: "parent", Status: "backlog", OwnerID: userA})
	child, _ := tasks.Create(ctx, CreateTaskParams{ID: uuid.New().String(), Title: "child", Status: "backlog", OwnerID: userA, ParentTaskID: &parent.ID})
	grand, _ := tasks.Create(ctx, CreateTaskParams{ID: uuid.New().String(), Title: "grand", Status: "backlog", OwnerID: userA, ParentTaskID: &child.ID})

	if err := tasks.SetDescendantsProject(ctx, parent.ID, &p.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{child.ID, grand.ID} {
		got, _ := tasks.Get(ctx, id, userA)
		if got.ProjectID == nil || *got.ProjectID != p.ID {
			t.Fatalf("%s project = %v, want %s", id, got.ProjectID, p.ID)
		}
	}
	if err := tasks.SetDescendantsProject(ctx, parent.ID, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := tasks.Get(ctx, grand.ID, userA)
	if got.ProjectID != nil {
		t.Fatalf("grand project = %v, want nil", *got.ProjectID)
	}
}
