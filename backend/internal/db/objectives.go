package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Objective struct {
	ID          string  `json:"id"`
	WeekStart   string  `json:"week_start"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	Position    float64 `json:"position"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	OwnerID     string  `json:"owner_id"`
	Shared      bool    `json:"shared"`
	ProjectID   *string `json:"project_id"` // migration 028
}

const objCols = `id, week_start, title, description, status, position, created_at, updated_at, owner_id, shared, project_id`

func scanObjective(s scanner) (Objective, error) {
	var o Objective
	var description, projectID sql.NullString
	var shared int64
	err := s.Scan(&o.ID, &o.WeekStart, &o.Title, &description, &o.Status, &o.Position, &o.CreatedAt, &o.UpdatedAt, &o.OwnerID, &shared, &projectID)
	if err != nil {
		return Objective{}, err
	}
	o.Description = nullStr(description)
	o.Shared = shared == 1
	o.ProjectID = nullStr(projectID)
	return o, nil
}

type ObjectiveStore struct{ db *sql.DB }

func NewObjectiveStore(db *sql.DB) *ObjectiveStore { return &ObjectiveStore{db: db} }

func (s *ObjectiveStore) ListByWeek(ctx context.Context, weekStart, ownerID string) ([]Objective, error) {
	scope, sargs := visScope(ownerID)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+objCols+` FROM weekly_objectives WHERE week_start = ?`+scope+` ORDER BY position`,
		append([]any{weekStart}, sargs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var objs []Objective
	for rows.Next() {
		o, err := scanObjective(rows)
		if err != nil {
			return nil, fmt.Errorf("scan objective: %w", err)
		}
		objs = append(objs, o)
	}
	if objs == nil {
		objs = []Objective{}
	}
	return objs, rows.Err()
}

// ListByProject returns every objective in a project, across all weeks.
func (s *ObjectiveStore) ListByProject(ctx context.Context, projectID, ownerID string) ([]Objective, error) {
	scope, sargs := visScope(ownerID)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+objCols+` FROM weekly_objectives WHERE project_id = ?`+scope+` ORDER BY week_start, position`,
		append([]any{projectID}, sargs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objs := []Objective{}
	for rows.Next() {
		o, err := scanObjective(rows)
		if err != nil {
			return nil, fmt.Errorf("scan objective: %w", err)
		}
		objs = append(objs, o)
	}
	return objs, rows.Err()
}

func (s *ObjectiveStore) Get(ctx context.Context, id, ownerID string) (Objective, error) {
	scope, sargs := visScope(ownerID)
	row := s.db.QueryRowContext(ctx, `SELECT `+objCols+` FROM weekly_objectives WHERE id = ?`+scope,
		append([]any{id}, sargs...)...)
	o, err := scanObjective(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Objective{}, ErrNotFound
	}
	return o, err
}

type CreateObjectiveParams struct {
	ID          string
	WeekStart   string
	Title       string
	Description *string
	Status      string
	Position    float64
	OwnerID     string
	Shared      bool
	ProjectID   *string
}

func (s *ObjectiveStore) Create(ctx context.Context, p CreateObjectiveParams) (Objective, error) {
	owner, err := resolveOwner(ctx, s.db, p.OwnerID)
	if err != nil {
		return Objective{}, err
	}
	shared := 0
	if p.Shared {
		shared = 1
	}
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO weekly_objectives (id, week_start, title, description, status, position, owner_id, shared, project_id)
		 VALUES (?,?,?,?,?,?,?,?,?) RETURNING `+objCols,
		p.ID, p.WeekStart, p.Title, p.Description, p.Status, p.Position, owner, shared, p.ProjectID,
	)
	return scanObjective(row)
}

func (s *ObjectiveStore) Update(ctx context.Context, o Objective) (Objective, error) {
	shared := 0
	if o.Shared {
		shared = 1
	}
	row := s.db.QueryRowContext(ctx, `
		UPDATE weekly_objectives SET
			week_start  = ?,
			title       = ?,
			description = ?,
			status      = ?,
			position    = ?,
			shared      = ?,
			project_id  = ?,
			updated_at  = datetime('now')
		WHERE id = ?
		RETURNING `+objCols,
		o.WeekStart, o.Title, o.Description, o.Status, o.Position, shared, o.ProjectID, o.ID,
	)
	updated, err := scanObjective(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Objective{}, ErrNotFound
	}
	return updated, err
}

func (s *ObjectiveStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM weekly_objectives WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
