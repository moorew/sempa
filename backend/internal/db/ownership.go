package db

import (
	"context"
	"database/sql"
	"fmt"
)

// SystemScope is the explicit owner value background callers (recurrence poller,
// reminder loop, integration sync, CalDAV export) pass to owner-scoped read
// methods to mean "all users — no scoping". It is a value no real user id can
// equal. An EMPTY owner id, by contrast, fails closed (matches only the
// long-gone unowned rows), so a handler bug can never leak everyone's data.
const SystemScope = "\x00system\x00"

// visScope returns the SQL fragment + arg that restricts a *shareable* table to
// rows the given user may see: their own, plus anything explicitly shared.
// SystemScope disables the filter. The fragment is always prefixed " AND " so it
// appends after an existing WHERE clause.
func visScope(ownerID string) (string, []any) {
	if ownerID == SystemScope {
		return "", nil
	}
	return ` AND (owner_id = ? OR shared = 1)`, []any{ownerID}
}

// ownScope is visScope for *personal* tables (daily_plans, week_reviews,
// pomodoro_sessions) that are never shared — owner-only.
func ownScope(ownerID string) (string, []any) {
	if ownerID == SystemScope {
		return "", nil
	}
	return ` AND owner_id = ?`, []any{ownerID}
}

// resolveOwner picks the owner id to stamp on a newly-created row. A non-empty
// explicit owner (from the request's user) wins; an empty owner — the case for
// background/integration creates that don't carry a user — falls back to the
// primary (server-owner) user so imported rows are never left invisibly
// unowned. SystemScope is treated as empty here.
func resolveOwner(ctx context.Context, database *sql.DB, ownerID string) (string, error) {
	if ownerID != "" && ownerID != SystemScope {
		return ownerID, nil
	}
	return PrimaryUserID(ctx, database)
}

// ownedTables are the per-user entities that carry an owner_id (migration 024).
// Tags stay global; attachments inherit ownership through their parent.
var ownedTables = []string{
	"tasks",
	"lists",
	"list_items",
	"weekly_objectives",
	"daily_plans",
	"week_reviews",
	"pomodoro_sessions",
}

// PrimaryUserID returns the oldest user's id — the account that existed before
// multi-user and therefore owns all the pre-existing data. Empty string if there
// are no users yet (a fresh or auth-disabled instance).
func PrimaryUserID(ctx context.Context, database *sql.DB) (string, error) {
	var id string
	err := database.QueryRowContext(ctx,
		`SELECT id FROM users ORDER BY created_at, id LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// BackfillOwnership assigns every still-unowned row (owner_id = '') to the
// primary user, so the existing single user keeps all their data once scoping
// turns on. Idempotent: it only touches '' rows, so re-running after more users
// exist never reassigns already-owned data. No-op when there are no users.
//
// Returns the number of rows claimed, for the startup log.
func BackfillOwnership(ctx context.Context, database *sql.DB) (int64, error) {
	primary, err := PrimaryUserID(ctx, database)
	if err != nil {
		return 0, fmt.Errorf("find primary user: %w", err)
	}
	if primary == "" {
		return 0, nil // nobody to own anything yet
	}

	var total int64
	for _, table := range ownedTables {
		res, err := database.ExecContext(ctx,
			// table names are from the fixed ownedTables literal, never user input
			fmt.Sprintf(`UPDATE %s SET owner_id = ? WHERE owner_id = ''`, table), primary)
		if err != nil {
			return total, fmt.Errorf("backfill %s: %w", table, err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}
