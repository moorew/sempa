package db

import (
	"context"
	"database/sql"
	"fmt"
)

type PomodoroSession struct {
	ID              string  `json:"id"`
	TaskID          string  `json:"task_id"`
	DurationMinutes int     `json:"duration_minutes"`
	StartedAt       string  `json:"started_at"`
	CompletedAt     *string `json:"completed_at"`
	WasCompleted    bool    `json:"was_completed"`
	CreatedAt       string  `json:"created_at"`
}

const sessionCols = `id, task_id, duration_minutes, started_at, completed_at, was_completed, created_at`

func scanSession(s scanner) (PomodoroSession, error) {
	var ps PomodoroSession
	var completedAt sql.NullString
	var wasCompleted int64

	err := s.Scan(
		&ps.ID, &ps.TaskID, &ps.DurationMinutes,
		&ps.StartedAt, &completedAt, &wasCompleted, &ps.CreatedAt,
	)
	if err != nil {
		return PomodoroSession{}, err
	}

	ps.CompletedAt = nullStr(completedAt)
	ps.WasCompleted = wasCompleted == 1
	return ps, nil
}

type SessionStore struct{ db *sql.DB }

func NewSessionStore(db *sql.DB) *SessionStore { return &SessionStore{db: db} }

type CreateSessionParams struct {
	ID              string
	TaskID          string
	DurationMinutes int
	StartedAt       string
	CompletedAt     *string
	WasCompleted    bool
}

func (s *SessionStore) Create(ctx context.Context, p CreateSessionParams) (PomodoroSession, error) {
	wasCompleted := 0
	if p.WasCompleted {
		wasCompleted = 1
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO pomodoro_sessions (id, task_id, duration_minutes, started_at, completed_at, was_completed)
		VALUES (?,?,?,?,?,?)
		RETURNING `+sessionCols,
		p.ID, p.TaskID, p.DurationMinutes, p.StartedAt, p.CompletedAt, wasCompleted,
	)
	return scanSession(row)
}

func (s *SessionStore) ListByTask(ctx context.Context, taskID string) ([]PomodoroSession, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+sessionCols+` FROM pomodoro_sessions WHERE task_id = ? ORDER BY started_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []PomodoroSession
	for rows.Next() {
		ps, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, ps)
	}
	if sessions == nil {
		sessions = []PomodoroSession{}
	}
	return sessions, rows.Err()
}
