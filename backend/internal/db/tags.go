package db

import (
	"context"
	"database/sql"
	"errors"
)

type TagDefinition struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type TagStore struct{ db *sql.DB }

func NewTagStore(db *sql.DB) *TagStore { return &TagStore{db: db} }

const tagCols = `id, name, color, created_at, updated_at`

func scanTag(s scanner) (TagDefinition, error) {
	var t TagDefinition
	err := s.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (s *TagStore) List(ctx context.Context) ([]TagDefinition, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tagCols+` FROM tag_definitions ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []TagDefinition
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []TagDefinition{}
	}
	return tags, rows.Err()
}

func (s *TagStore) Upsert(ctx context.Context, id, name, color string) (TagDefinition, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO tag_definitions (id, name, color)
		VALUES (?, ?, ?)
		ON CONFLICT(lower(name)) DO UPDATE SET
			color      = excluded.color,
			updated_at = datetime('now')
		RETURNING `+tagCols,
		id, name, color,
	)
	return scanTag(row)
}

func (s *TagStore) Update(ctx context.Context, id, color string) (TagDefinition, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE tag_definitions SET color = ?, updated_at = datetime('now')
		WHERE id = ?
		RETURNING `+tagCols,
		color, id,
	)
	t, err := scanTag(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TagDefinition{}, ErrNotFound
	}
	return t, err
}

func (s *TagStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tag_definitions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *TagStore) EnsureExists(ctx context.Context, id, name, color string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tag_definitions (id, name, color) VALUES (?, ?, ?)
		ON CONFLICT(lower(name)) DO NOTHING`,
		id, name, color,
	)
	return err
}

// BulkEnsure creates tag definitions for any names that don't exist yet, using the provided color palette.
func (s *TagStore) BulkEnsure(ctx context.Context, names []string, palette []string) error {
	existing := map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT lower(name) FROM tag_definitions`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return err
		}
		existing[n] = true
	}

	paletteIdx := len(existing) % len(palette)
	for _, name := range names {
		if existing[name] {
			continue
		}
		color := palette[paletteIdx%len(palette)]
		paletteIdx++
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO tag_definitions (id, name, color) VALUES (lower(?), lower(?), ?)
			ON CONFLICT(lower(name)) DO NOTHING`,
			name, name, color,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
