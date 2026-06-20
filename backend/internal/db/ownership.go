package db

import (
	"context"
	"database/sql"
	"fmt"
)

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
