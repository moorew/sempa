package db

import (
	"context"
	"database/sql"
)

// List is a standalone checklist, optionally linked to a task.
type List struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	TaskID            *string `json:"task_id"`
	Position          float64 `json:"position"`
	ArchivedAt        *string `json:"archived_at"`
	ArchiveOnComplete bool    `json:"archive_on_complete"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
	OwnerID           string  `json:"owner_id"`
	Shared            bool    `json:"shared"`
}

// ListItem is one row in a list. `Done` greys/strikes it in place; `Category` is
// set by Organize-with-AI to group items. owner_id/shared are denormalised from
// the parent list (kept in lockstep) so item scoping is uniform with everything else.
type ListItem struct {
	ID        string  `json:"id"`
	ListID    string  `json:"list_id"`
	Text      string  `json:"text"`
	Position  float64 `json:"position"`
	Done      bool    `json:"done"`
	Category  *string `json:"category"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	OwnerID   string  `json:"owner_id"`
	Shared    bool    `json:"shared"`
}

const listCols = `id, name, task_id, position, archived_at, archive_on_complete, created_at, updated_at, owner_id, shared`
const listItemCols = `id, list_id, text, position, done, category, created_at, updated_at, owner_id, shared`

func scanList(s scanner) (List, error) {
	var l List
	var taskID, archivedAt sql.NullString
	var archiveOnComplete, shared int64
	if err := s.Scan(&l.ID, &l.Name, &taskID, &l.Position, &archivedAt, &archiveOnComplete, &l.CreatedAt, &l.UpdatedAt, &l.OwnerID, &shared); err != nil {
		return List{}, err
	}
	l.TaskID = nullStr(taskID)
	l.ArchivedAt = nullStr(archivedAt)
	l.ArchiveOnComplete = archiveOnComplete == 1
	l.Shared = shared == 1
	return l, nil
}

func scanListItem(s scanner) (ListItem, error) {
	var it ListItem
	var category sql.NullString
	var done, shared int64
	if err := s.Scan(&it.ID, &it.ListID, &it.Text, &it.Position, &done, &category, &it.CreatedAt, &it.UpdatedAt, &it.OwnerID, &shared); err != nil {
		return ListItem{}, err
	}
	it.Done = done == 1
	it.Category = nullStr(category)
	it.Shared = shared == 1
	return it, nil
}

type ListStore struct{ db *sql.DB }

func NewListStore(db *sql.DB) *ListStore { return &ListStore{db: db} }

// ── Lists ────────────────────────────────────────────────────────────────────

func (s *ListStore) List(ctx context.Context, includeArchived bool, taskID, ownerID string) ([]List, error) {
	q := `SELECT ` + listCols + ` FROM lists`
	var where []string
	var args []any
	if !includeArchived {
		where = append(where, "archived_at IS NULL")
	}
	if taskID != "" {
		where = append(where, "task_id = ?")
		args = append(args, taskID)
	}
	// Visibility: own + shared. Expressed as a where-term so it composes with the
	// optional filters above.
	if scope, sargs := visScope(ownerID); scope != "" {
		where = append(where, "(owner_id = ? OR shared = 1)")
		args = append(args, sargs...)
	}
	for i, w := range where {
		if i == 0 {
			q += " WHERE "
		} else {
			q += " AND "
		}
		q += w
	}
	q += " ORDER BY position, created_at"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []List{}
	for rows.Next() {
		l, err := scanList(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *ListStore) Get(ctx context.Context, id, ownerID string) (List, error) {
	scope, sargs := visScope(ownerID)
	return scanList(s.db.QueryRowContext(ctx,
		`SELECT `+listCols+` FROM lists WHERE id = ?`+scope, append([]any{id}, sargs...)...))
}

type CreateListParams struct {
	ID      string
	Name    string
	TaskID  *string
	OwnerID string
	Shared  bool
}

func (s *ListStore) Create(ctx context.Context, p CreateListParams) (List, error) {
	owner, err := resolveOwner(ctx, s.db, p.OwnerID)
	if err != nil {
		return List{}, err
	}
	shared := 0
	if p.Shared {
		shared = 1
	}
	return scanList(s.db.QueryRowContext(ctx, `
		INSERT INTO lists (id, name, task_id, position, owner_id, shared)
		VALUES (?,?,?, COALESCE((SELECT MAX(position)+1 FROM lists), 0), ?, ?)
		RETURNING `+listCols, p.ID, p.Name, p.TaskID, owner, shared))
}

// Update saves the mutable fields of a list (caller passes the desired values),
// including the shared flag.
func (s *ListStore) Update(ctx context.Context, l List) (List, error) {
	archiveOnComplete := 0
	if l.ArchiveOnComplete {
		archiveOnComplete = 1
	}
	shared := 0
	if l.Shared {
		shared = 1
	}
	return scanList(s.db.QueryRowContext(ctx, `
		UPDATE lists SET name = ?, task_id = ?, position = ?, archived_at = ?,
		    archive_on_complete = ?, shared = ?, updated_at = datetime('now')
		WHERE id = ?
		RETURNING `+listCols,
		l.Name, l.TaskID, l.Position, l.ArchivedAt, archiveOnComplete, shared, l.ID))
}

// SetItemsShared cascades a list's share state to all its items, so item
// visibility stays in lockstep with the parent (items bump updated_at to sync).
func (s *ListStore) SetItemsShared(ctx context.Context, listID string, shared bool) error {
	v := 0
	if shared {
		v = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE list_items SET shared = ?, updated_at = datetime('now') WHERE list_id = ? AND shared != ?`,
		v, listID, v)
	return err
}

func (s *ListStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM lists WHERE id = ?`, id)
	return err
}

// ArchiveForCompletedTask archives lists that opted in, when their task is done.
// Returns the affected list ids so the caller can record sync tombstones... no —
// archiving keeps the row (sets archived_at), so it syncs as an update, not a
// deletion. Returns ids purely for SSE/diagnostics.
func (s *ListStore) ArchiveForCompletedTask(ctx context.Context, taskID, ownerID string) ([]string, error) {
	scope, sargs := visScope(ownerID)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM lists WHERE task_id = ? AND archive_on_complete = 1 AND archived_at IS NULL`+scope,
		append([]any{taskID}, sargs...)...)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, nil
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE lists SET archived_at = datetime('now'), updated_at = datetime('now')
		 WHERE task_id = ? AND archive_on_complete = 1 AND archived_at IS NULL`+scope,
		append([]any{taskID}, sargs...)...)
	return ids, err
}

// ── Items ────────────────────────────────────────────────────────────────────

func (s *ListStore) Items(ctx context.Context, listID, ownerID string) ([]ListItem, error) {
	scope, sargs := visScope(ownerID)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+listItemCols+` FROM list_items WHERE list_id = ?`+scope+` ORDER BY position, created_at`,
		append([]any{listID}, sargs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListItem{}
	for rows.Next() {
		it, err := scanListItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *ListStore) GetItem(ctx context.Context, id, ownerID string) (ListItem, error) {
	scope, sargs := visScope(ownerID)
	return scanListItem(s.db.QueryRowContext(ctx,
		`SELECT `+listItemCols+` FROM list_items WHERE id = ?`+scope, append([]any{id}, sargs...)...))
}

type CreateItemParams struct {
	ID     string
	ListID string
	Text   string
}

// CreateItem inherits owner_id + shared straight from the parent list, so an
// item is always exactly as visible as the list that holds it.
func (s *ListStore) CreateItem(ctx context.Context, p CreateItemParams) (ListItem, error) {
	return scanListItem(s.db.QueryRowContext(ctx, `
		INSERT INTO list_items (id, list_id, text, position, owner_id, shared)
		VALUES (?,?,?,
		        COALESCE((SELECT MAX(position)+1 FROM list_items WHERE list_id = ?), 0),
		        COALESCE((SELECT owner_id FROM lists WHERE id = ?), ''),
		        COALESCE((SELECT shared   FROM lists WHERE id = ?), 0))
		RETURNING `+listItemCols, p.ID, p.ListID, p.Text, p.ListID, p.ListID, p.ListID))
}

func (s *ListStore) UpdateItem(ctx context.Context, it ListItem) (ListItem, error) {
	done := 0
	if it.Done {
		done = 1
	}
	return scanListItem(s.db.QueryRowContext(ctx, `
		UPDATE list_items SET text = ?, position = ?, done = ?, category = ?, updated_at = datetime('now')
		WHERE id = ?
		RETURNING `+listItemCols, it.Text, it.Position, done, it.Category, it.ID))
}

func (s *ListStore) DeleteItem(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM list_items WHERE id = ?`, id)
	return err
}

// Reorder sets positions for a batch of item ids (index order). Touches
// updated_at so the new order syncs.
func (s *ListStore) Reorder(ctx context.Context, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE list_items SET position = ?, updated_at = datetime('now') WHERE id = ?`,
			float64(i), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
