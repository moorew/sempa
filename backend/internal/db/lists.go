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
}

// ListItem is one row in a list. `Done` greys/strikes it in place; `Category` is
// set by Organize-with-AI to group items.
type ListItem struct {
	ID        string  `json:"id"`
	ListID    string  `json:"list_id"`
	Text      string  `json:"text"`
	Position  float64 `json:"position"`
	Done      bool    `json:"done"`
	Category  *string `json:"category"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

const listCols = `id, name, task_id, position, archived_at, archive_on_complete, created_at, updated_at`
const listItemCols = `id, list_id, text, position, done, category, created_at, updated_at`

func scanList(s scanner) (List, error) {
	var l List
	var taskID, archivedAt sql.NullString
	var archiveOnComplete int64
	if err := s.Scan(&l.ID, &l.Name, &taskID, &l.Position, &archivedAt, &archiveOnComplete, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return List{}, err
	}
	l.TaskID = nullStr(taskID)
	l.ArchivedAt = nullStr(archivedAt)
	l.ArchiveOnComplete = archiveOnComplete == 1
	return l, nil
}

func scanListItem(s scanner) (ListItem, error) {
	var it ListItem
	var category sql.NullString
	var done int64
	if err := s.Scan(&it.ID, &it.ListID, &it.Text, &it.Position, &done, &category, &it.CreatedAt, &it.UpdatedAt); err != nil {
		return ListItem{}, err
	}
	it.Done = done == 1
	it.Category = nullStr(category)
	return it, nil
}

type ListStore struct{ db *sql.DB }

func NewListStore(db *sql.DB) *ListStore { return &ListStore{db: db} }

// ── Lists ────────────────────────────────────────────────────────────────────

func (s *ListStore) List(ctx context.Context, includeArchived bool, taskID string) ([]List, error) {
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

func (s *ListStore) Get(ctx context.Context, id string) (List, error) {
	return scanList(s.db.QueryRowContext(ctx, `SELECT `+listCols+` FROM lists WHERE id = ?`, id))
}

type CreateListParams struct {
	ID     string
	Name   string
	TaskID *string
}

func (s *ListStore) Create(ctx context.Context, p CreateListParams) (List, error) {
	return scanList(s.db.QueryRowContext(ctx, `
		INSERT INTO lists (id, name, task_id, position)
		VALUES (?,?,?, COALESCE((SELECT MAX(position)+1 FROM lists), 0))
		RETURNING `+listCols, p.ID, p.Name, p.TaskID))
}

// Update saves the mutable fields of a list (caller passes the desired values).
func (s *ListStore) Update(ctx context.Context, l List) (List, error) {
	archiveOnComplete := 0
	if l.ArchiveOnComplete {
		archiveOnComplete = 1
	}
	return scanList(s.db.QueryRowContext(ctx, `
		UPDATE lists SET name = ?, task_id = ?, position = ?, archived_at = ?,
		    archive_on_complete = ?, updated_at = datetime('now')
		WHERE id = ?
		RETURNING `+listCols,
		l.Name, l.TaskID, l.Position, l.ArchivedAt, archiveOnComplete, l.ID))
}

func (s *ListStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM lists WHERE id = ?`, id)
	return err
}

// ArchiveForCompletedTask archives lists that opted in, when their task is done.
// Returns the affected list ids so the caller can record sync tombstones... no —
// archiving keeps the row (sets archived_at), so it syncs as an update, not a
// deletion. Returns ids purely for SSE/diagnostics.
func (s *ListStore) ArchiveForCompletedTask(ctx context.Context, taskID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM lists WHERE task_id = ? AND archive_on_complete = 1 AND archived_at IS NULL`, taskID)
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
		 WHERE task_id = ? AND archive_on_complete = 1 AND archived_at IS NULL`, taskID)
	return ids, err
}

// ── Items ────────────────────────────────────────────────────────────────────

func (s *ListStore) Items(ctx context.Context, listID string) ([]ListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+listItemCols+` FROM list_items WHERE list_id = ? ORDER BY position, created_at`, listID)
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

func (s *ListStore) GetItem(ctx context.Context, id string) (ListItem, error) {
	return scanListItem(s.db.QueryRowContext(ctx, `SELECT `+listItemCols+` FROM list_items WHERE id = ?`, id))
}

type CreateItemParams struct {
	ID     string
	ListID string
	Text   string
}

func (s *ListStore) CreateItem(ctx context.Context, p CreateItemParams) (ListItem, error) {
	return scanListItem(s.db.QueryRowContext(ctx, `
		INSERT INTO list_items (id, list_id, text, position)
		VALUES (?,?,?, COALESCE((SELECT MAX(position)+1 FROM list_items WHERE list_id = ?), 0))
		RETURNING `+listItemCols, p.ID, p.ListID, p.Text, p.ListID))
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
