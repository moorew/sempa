package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBackfillOwnership_ClaimsUnownedRowsForOldestUser(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(filepath.Join(t.TempDir(), "owner.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := NewUserStore(conn)

	// The "husband" — created first, so the primary user.
	primary, err := users.Create(ctx, CreateUserParams{Email: "husband@example.com", IsAdmin: true})
	if err != nil {
		t.Fatalf("create primary: %v", err)
	}
	// A later user who must NOT inherit existing data.
	if _, err := users.Create(ctx, CreateUserParams{Email: "wife@example.com"}); err != nil {
		t.Fatalf("create wife: %v", err)
	}
	// Both Create calls land in the same clock-second, so pin an explicit older
	// timestamp on the primary — in production their created_at differs (the
	// second user is added days later), and backfill runs while only one exists.
	if _, err := conn.ExecContext(ctx,
		`UPDATE users SET created_at = '2020-01-01 00:00:00' WHERE id = ?`, primary.ID); err != nil {
		t.Fatalf("age primary: %v", err)
	}

	// Seed pre-multi-user rows (owner_id defaults to '').
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO tasks (id, title) VALUES ('t1','a'), ('t2','b')`); err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO lists (id, name) VALUES ('l1','groceries')`); err != nil {
		t.Fatalf("seed list: %v", err)
	}

	n, err := BackfillOwnership(ctx, conn)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 rows claimed, got %d", n)
	}

	// Everything seeded now belongs to the primary user, none to the wife.
	var owned int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE owner_id = ?`, primary.ID).Scan(&owned); err != nil {
		t.Fatalf("count: %v", err)
	}
	if owned != 2 {
		t.Fatalf("expected 2 tasks owned by primary, got %d", owned)
	}

	// Idempotent: a second run claims nothing.
	again, err := BackfillOwnership(ctx, conn)
	if err != nil {
		t.Fatalf("backfill again: %v", err)
	}
	if again != 0 {
		t.Fatalf("expected idempotent re-run to claim 0, got %d", again)
	}
}

func TestBackfillOwnership_NoUsersIsNoOp(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(filepath.Join(t.TempDir(), "owner.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO tasks (id, title) VALUES ('t1','a')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	n, err := BackfillOwnership(ctx, conn)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected no-op with no users, got %d claimed", n)
	}
}
