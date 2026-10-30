package db

import (
	"context"
	"database/sql"
	"errors"
)

// Project is a long-lived container (like a Jira epic) for tasks, weekly
// objectives and lists. See migration 028_projects.sql.
type Project struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	StatusID    string  `json:"status_id"`
	Color       string  `json:"color"`
	StartDate   *string `json:"start_date"`
	TargetDate  *string `json:"target_date"`
	Position    float64 `json:"position"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	OwnerID     string  `json:"owner_id"`
	Shared      bool    `json:"shared"`
}

// ProjectStatus is a user-customisable project status. Category is one of the
// fixed ProjectCategories and is what app logic keys off; name/color/position
// are free for the user to change.
type ProjectStatus struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Color     string  `json:"color"`
	Category  string  `json:"category"`
	Position  float64 `json:"position"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ProjectCategories are the fixed buckets every status maps to.
var ProjectCategories = []string{"planning", "active", "on_hold", "done", "archived"}

// ValidProjectCategory reports whether c is one of ProjectCategories.
func ValidProjectCategory(c string) bool {
	for _, k := range ProjectCategories {
		if k == c {
			return true
		}
	}
	return false
}

const projectCols = `id, title, description, status_id, color, start_date, target_date, position, created_at, updated_at, owner_id, shared`
const projectStatusCols = `id, name, color, category, position, created_at, updated_at`

func scanProject(s scanner) (Project, error) {
	var p Project
	var description, startDate, targetDate sql.NullString
	var shared int64
	if err := s.Scan(&p.ID, &p.Title, &description, &p.StatusID, &p.Color, &startDate, &targetDate,
		&p.Position, &p.CreatedAt, &p.UpdatedAt, &p.OwnerID, &shared); err != nil {
		return Project{}, err
	}
	p.Description = nullStr(description)
	p.StartDate = nullStr(startDate)
	p.TargetDate = nullStr(targetDate)
	p.Shared = shared == 1
	return p, nil
}

func scanProjectStatus(s scanner) (ProjectStatus, error) {
	var st ProjectStatus
	err := s.Scan(&st.ID, &st.Name, &st.Color, &st.Category, &st.Position, &st.CreatedAt, &st.UpdatedAt)
	return st, err
}

type ProjectStore struct{ db *sql.DB }

func NewProjectStore(db *sql.DB) *ProjectStore { return &ProjectStore{db: db} }

// ── Projects ─────────────────────────────────────────────────────────────────

func (s *ProjectStore) List(ctx context.Context, ownerID string) ([]Project, error) {
	scope, sargs := visScope(ownerID)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+projectCols+` FROM projects WHERE 1=1`+scope+` ORDER BY position, created_at`, sargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *ProjectStore) Get(ctx context.Context, id, ownerID string) (Project, error) {
	scope, sargs := visScope(ownerID)
	p, err := scanProject(s.db.QueryRowContext(ctx,
		`SELECT `+projectCols+` FROM projects WHERE id = ?`+scope, append([]any{id}, sargs...)...))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

type CreateProjectParams struct {
	ID          string
	Title       string
	Description *string
	StatusID    string
	Color       string
	StartDate   *string
	TargetDate  *string
	OwnerID     string
	Shared      bool
}

func (s *ProjectStore) Create(ctx context.Context, p CreateProjectParams) (Project, error) {
	owner, err := resolveOwner(ctx, s.db, p.OwnerID)
	if err != nil {
		return Project{}, err
	}
	shared := 0
	if p.Shared {
		shared = 1
	}
	if p.StatusID == "" {
		p.StatusID = s.defaultStatusID(ctx)
	}
	if p.Color == "" {
		p.Color = "#b3592e"
	}
	return scanProject(s.db.QueryRowContext(ctx, `
		INSERT INTO projects (id, title, description, status_id, color, start_date, target_date, position, owner_id, shared)
		VALUES (?,?,?,?,?,?,?, COALESCE((SELECT MAX(position)+1 FROM projects), 0), ?, ?)
		RETURNING `+projectCols,
		p.ID, p.Title, p.Description, p.StatusID, p.Color, p.StartDate, p.TargetDate, owner, shared))
}

// Update saves the mutable fields of a project (caller passes the desired values).
func (s *ProjectStore) Update(ctx context.Context, p Project) (Project, error) {
	shared := 0
	if p.Shared {
		shared = 1
	}
	updated, err := scanProject(s.db.QueryRowContext(ctx, `
		UPDATE projects SET title = ?, description = ?, status_id = ?, color = ?,
		    start_date = ?, target_date = ?, position = ?, shared = ?, updated_at = datetime('now')
		WHERE id = ?
		RETURNING `+projectCols,
		p.Title, p.Description, p.StatusID, p.Color, p.StartDate, p.TargetDate, p.Position, shared, p.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return updated, err
}

// ProjectLinks counts the rows a project delete unlinked.
type ProjectLinks struct {
	Tasks      int `json:"tasks"`
	Objectives int `json:"objectives"`
	Lists      int `json:"lists"`
}

// Delete removes a project and unlinks (never deletes) its tasks, objectives and
// lists. The FK's ON DELETE SET NULL would do the unlink too, but it doesn't
// bump updated_at — so offline clients would keep a dangling project_id. Clear
// them explicitly first, in one transaction.
func (s *ProjectStore) Delete(ctx context.Context, id string) (ProjectLinks, error) {
	var links ProjectLinks
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return links, err
	}
	defer tx.Rollback()
	for _, t := range []struct {
		table string
		n     *int
	}{{"tasks", &links.Tasks}, {"weekly_objectives", &links.Objectives}, {"lists", &links.Lists}} {
		// table names are from the fixed literal above, never user input
		res, err := tx.ExecContext(ctx,
			`UPDATE `+t.table+` SET project_id = NULL, updated_at = datetime('now') WHERE project_id = ?`, id)
		if err != nil {
			return links, err
		}
		n, _ := res.RowsAffected()
		*t.n = int(n)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return links, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return links, ErrNotFound
	}
	return links, tx.Commit()
}

// ── Project statuses ─────────────────────────────────────────────────────────

func (s *ProjectStore) Statuses(ctx context.Context) ([]ProjectStatus, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+projectStatusCols+` FROM project_statuses ORDER BY position, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProjectStatus{}
	for rows.Next() {
		st, err := scanProjectStatus(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *ProjectStore) GetStatus(ctx context.Context, id string) (ProjectStatus, error) {
	st, err := scanProjectStatus(s.db.QueryRowContext(ctx,
		`SELECT `+projectStatusCols+` FROM project_statuses WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectStatus{}, ErrNotFound
	}
	return st, err
}

// defaultStatusID is the status a new project gets: the first 'active' status,
// else the first status of any kind, else the seeded 'active' id.
func (s *ProjectStore) defaultStatusID(ctx context.Context) string {
	var id string
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM project_statuses
		ORDER BY (category = 'active') DESC, position, created_at LIMIT 1`).Scan(&id)
	if err != nil {
		return "active"
	}
	return id
}

func (s *ProjectStore) CreateStatus(ctx context.Context, st ProjectStatus) (ProjectStatus, error) {
	return scanProjectStatus(s.db.QueryRowContext(ctx, `
		INSERT INTO project_statuses (id, name, color, category, position)
		VALUES (?,?,?,?, COALESCE((SELECT MAX(position)+1 FROM project_statuses), 0))
		RETURNING `+projectStatusCols,
		st.ID, st.Name, st.Color, st.Category))
}

func (s *ProjectStore) UpdateStatus(ctx context.Context, st ProjectStatus) (ProjectStatus, error) {
	updated, err := scanProjectStatus(s.db.QueryRowContext(ctx, `
		UPDATE project_statuses SET name = ?, color = ?, category = ?, position = ?, updated_at = datetime('now')
		WHERE id = ?
		RETURNING `+projectStatusCols,
		st.Name, st.Color, st.Category, st.Position, st.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectStatus{}, ErrNotFound
	}
	return updated, err
}

// ErrLastStatus is returned when deleting the only remaining status.
var ErrLastStatus = errors.New("cannot delete the last project status")

// DeleteStatus removes a status and moves its projects to a replacement: the
// first remaining status in the same category, else the first remaining status.
// Moved projects get an updated_at bump so the reassignment syncs. Returns the
// replacement id.
func (s *ProjectStore) DeleteStatus(ctx context.Context, id string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var category string
	if err := tx.QueryRowContext(ctx,
		`SELECT category FROM project_statuses WHERE id = ?`, id).Scan(&category); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	var replacement string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM project_statuses WHERE id != ?
		ORDER BY (category = ?) DESC, position, created_at LIMIT 1`, id, category).Scan(&replacement)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrLastStatus
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE projects SET status_id = ?, updated_at = datetime('now') WHERE status_id = ?`,
		replacement, id); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_statuses WHERE id = ?`, id); err != nil {
		return "", err
	}
	return replacement, tx.Commit()
}

// ReorderStatuses sets positions for a batch of status ids (index order).
func (s *ProjectStore) ReorderStatuses(ctx context.Context, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE project_statuses SET position = ?, updated_at = datetime('now') WHERE id = ?`,
			float64(i), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
