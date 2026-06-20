package db

import (
	"context"
	"database/sql"
)

// Tombstone records a deleted entity so offline clients learn to drop their
// local copy when they next pull. See migration 015_sync_tombstones.sql.
type Tombstone struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	DeletedAt  string `json:"deleted_at"`
}

// SyncChanges is the payload returned by GET /api/v1/sync/changes?since=<cursor>.
// It carries every entity created/updated since the cursor, plus tombstones for
// deletions, and a new cursor the client persists for its next pull.
type SyncChanges struct {
	Tasks       []Task          `json:"tasks"`
	Objectives  []Objective     `json:"objectives"`
	Plans       []DailyPlan     `json:"plans"`
	Tags        []TagDefinition `json:"tags"`
	WeekReviews []WeekReview    `json:"week_reviews"`
	Lists       []List          `json:"lists"`
	ListItems   []ListItem      `json:"list_items"`
	Deletions   []Tombstone     `json:"deletions"`
	Cursor      string          `json:"cursor"`
}

// SyncStore powers the pull side of offline sync and records deletions.
type SyncStore struct{ db *sql.DB }

func NewSyncStore(db *sql.DB) *SyncStore { return &SyncStore{db: db} }

// RecordTombstone notes that an entity was deleted so the deletion propagates to
// offline clients on their next pull. Called from delete handlers. A re-created
// entity (same id) overwrites its tombstone via the primary-key upsert.
//
// ownerID + wasShared decide who the deletion is delivered to (see Changes):
// the owner always, plus everyone if it was shared or global (ownerID == "").
func (s *SyncStore) RecordTombstone(ctx context.Context, entityType, entityID, ownerID string, wasShared bool) error {
	shared := 0
	if wasShared {
		shared = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_tombstones (entity_type, entity_id, deleted_at, owner_id, was_shared, kind)
		VALUES (?, ?, datetime('now'), ?, ?, 'delete')
		ON CONFLICT(entity_type, entity_id) DO UPDATE SET
			deleted_at = datetime('now'), owner_id = excluded.owner_id,
			was_shared = excluded.was_shared, kind = 'delete'`,
		entityType, entityID, ownerID, shared)
	return err
}

// RecordRevocation marks that a previously-shared entity is now private to
// ownerID. It propagates to every OTHER user (so they drop their stale local
// copy) but not the owner, who still has it. The entity itself is not deleted.
func (s *SyncStore) RecordRevocation(ctx context.Context, entityType, entityID, ownerID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_tombstones (entity_type, entity_id, deleted_at, owner_id, was_shared, kind)
		VALUES (?, ?, datetime('now'), ?, 0, 'revoke')
		ON CONFLICT(entity_type, entity_id) DO UPDATE SET
			deleted_at = datetime('now'), owner_id = excluded.owner_id,
			was_shared = 0, kind = 'revoke'`,
		entityType, entityID, ownerID)
	return err
}

// ClearTombstone removes a tombstone — call when an entity with the same id is
// re-created, so a stale deletion doesn't later wipe the new row on a client.
func (s *SyncStore) ClearTombstone(ctx context.Context, entityType, entityID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sync_tombstones WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID)
	return err
}

// Changes returns everything modified since the given cursor. An empty cursor
// means "everything" (initial full sync). The returned Cursor is the server's
// current time; the client passes it back on the next pull.
//
// Comparisons are lexicographic on the `datetime('now')` text format
// (YYYY-MM-DD HH:MM:SS, UTC, fixed width) which all updated_at columns use, so
// string ordering equals chronological ordering. Resolution is one second;
// same-second concurrent writes are reconciled by the client's idempotent,
// id-keyed last-write-wins upsert and by the live SSE channel.
func (s *SyncStore) Changes(ctx context.Context, since, ownerID string) (SyncChanges, error) {
	out := SyncChanges{
		Tasks:       []Task{},
		Objectives:  []Objective{},
		Plans:       []DailyPlan{},
		Tags:        []TagDefinition{},
		WeekReviews: []WeekReview{},
		Lists:       []List{},
		ListItems:   []ListItem{},
		Deletions:   []Tombstone{},
	}

	// Capture the server clock up front; it becomes the next cursor.
	if err := s.db.QueryRowContext(ctx, `SELECT datetime('now')`).Scan(&out.Cursor); err != nil {
		return out, err
	}

	// build assembles "WHERE 1=1 [AND updated_at > ?] [AND <scope>]" so the cursor
	// filter and the per-table visibility scope compose cleanly. visScope is for
	// shareable tables (own + shared), ownScope for personal ones, and an empty
	// scope ("") leaves a table global (tags).
	build := func(scopeFrag string, scopeArgs []any) (string, []any) {
		q := " WHERE 1=1"
		a := []any{}
		if since != "" {
			q += " AND updated_at > ?"
			a = append(a, since)
		}
		q += scopeFrag
		a = append(a, scopeArgs...)
		return q, a
	}
	vsFrag, vsArgs := visScope(ownerID)
	owFrag, owArgs := ownScope(ownerID)

	// Tasks (shareable)
	tf, ta := build(vsFrag, vsArgs)
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskCols+` FROM tasks`+tf+` ORDER BY updated_at`, ta...); err != nil {
		return out, err
	} else {
		out.Tasks, err = collectTasks(rows)
		rows.Close()
		if err != nil {
			return out, err
		}
	}

	// Objectives (shareable)
	of, oa := build(vsFrag, vsArgs)
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+objCols+` FROM weekly_objectives`+of+` ORDER BY updated_at`, oa...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			o, err := scanObjective(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out.Objectives = append(out.Objectives, o)
		}
		rows.Close()
	}

	// Daily plans (personal)
	pf, pa := build(owFrag, owArgs)
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+planCols+` FROM daily_plans`+pf+` ORDER BY updated_at`, pa...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			p, err := scanPlan(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out.Plans = append(out.Plans, p)
		}
		rows.Close()
	}

	// Tags (global — no scope)
	gf, ga := build("", nil)
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+tagCols+` FROM tag_definitions`+gf+` ORDER BY updated_at`, ga...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			var t TagDefinition
			if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
				rows.Close()
				return out, err
			}
			out.Tags = append(out.Tags, t)
		}
		rows.Close()
	}

	// Week reviews (personal)
	wf, wa := build(owFrag, owArgs)
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+weekReviewCols+`
		 FROM week_reviews`+wf+` ORDER BY updated_at`, wa...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			r, err := scanWeekReview(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out.WeekReviews = append(out.WeekReviews, r)
		}
		rows.Close()
	}

	// Lists (shareable)
	lf, la := build(vsFrag, vsArgs)
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+listCols+` FROM lists`+lf+` ORDER BY updated_at`, la...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			l, err := scanList(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out.Lists = append(out.Lists, l)
		}
		rows.Close()
	}

	// List items (shareable; owner_id/shared denormalised from parent list)
	lif, lia := build(vsFrag, vsArgs)
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+listItemCols+` FROM list_items`+lif+` ORDER BY updated_at`, lia...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			it, err := scanListItem(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out.ListItems = append(out.ListItems, it)
		}
		rows.Close()
	}

	// Deletions — delivered per user. A 'delete' reaches the owner, plus everyone
	// when the row was shared or global (owner_id=''). A 'revoke' (un-share)
	// reaches everyone EXCEPT the owner, so peers drop their stale copy while the
	// owner keeps the now-private row. SystemScope sees all tombstones.
	delQ := `SELECT entity_type, entity_id, deleted_at FROM sync_tombstones WHERE 1=1`
	delArgs := []any{}
	if since != "" {
		delQ += ` AND deleted_at > ?`
		delArgs = append(delArgs, since)
	}
	if ownerID != SystemScope {
		delQ += ` AND ((kind = 'delete' AND (owner_id = '' OR owner_id = ? OR was_shared = 1))
		             OR (kind = 'revoke' AND owner_id != ?))`
		delArgs = append(delArgs, ownerID, ownerID)
	}
	delQ += ` ORDER BY deleted_at`
	if rows, err := s.db.QueryContext(ctx, delQ, delArgs...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			var t Tombstone
			if err := rows.Scan(&t.EntityType, &t.EntityID, &t.DeletedAt); err != nil {
				rows.Close()
				return out, err
			}
			out.Deletions = append(out.Deletions, t)
		}
		rows.Close()
	}

	return out, nil
}
