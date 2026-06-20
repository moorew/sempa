package db

import (
	"context"
	"database/sql"
	"errors"
)

type WeekReview struct {
	ID         string  `json:"id"`
	WeekStart  string  `json:"week_start"`
	Wins       *string `json:"wins"`       // JSON array of strings
	Challenges *string `json:"challenges"` // JSON array of strings
	NextFocus  *string `json:"next_focus"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	OwnerID    string  `json:"owner_id"`
}

const weekReviewCols = `id, week_start, wins, challenges, next_focus, created_at, updated_at, owner_id`

type WeekReviewStore struct {
	db *sql.DB
}

func NewWeekReviewStore(db *sql.DB) *WeekReviewStore {
	return &WeekReviewStore{db: db}
}

func (s *WeekReviewStore) Get(ctx context.Context, weekStart, ownerID string) (WeekReview, error) {
	scope, sargs := ownScope(ownerID)
	row := s.db.QueryRowContext(ctx,
		`SELECT `+weekReviewCols+` FROM week_reviews WHERE week_start = ?`+scope,
		append([]any{weekStart}, sargs...)...)
	return scanWeekReview(row)
}

// List returns all week reviews, newest first. limit <= 0 means no limit.
func (s *WeekReviewStore) List(ctx context.Context, limit int, ownerID string) ([]WeekReview, error) {
	scope, sargs := ownScope(ownerID)
	q := `SELECT ` + weekReviewCols + ` FROM week_reviews`
	if scope != "" {
		q += ` WHERE 1=1` + scope
	}
	q += ` ORDER BY week_start DESC`
	args := append([]any{}, sargs...)
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := []WeekReview{}
	for rows.Next() {
		r, err := scanWeekReview(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}

func (s *WeekReviewStore) Upsert(ctx context.Context, id, weekStart string, wins, challenges, nextFocus *string, ownerID string) (WeekReview, error) {
	owner, err := resolveOwner(ctx, s.db, ownerID)
	if err != nil {
		return WeekReview{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO week_reviews (id, week_start, wins, challenges, next_focus, owner_id)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(owner_id, week_start) DO UPDATE SET
			wins       = excluded.wins,
			challenges = excluded.challenges,
			next_focus = excluded.next_focus,
			updated_at = datetime('now')
		RETURNING `+weekReviewCols,
		id, weekStart, wins, challenges, nextFocus, owner,
	)
	return scanWeekReview(row)
}

func scanWeekReview(s scanner) (WeekReview, error) {
	var r WeekReview
	var wins, challenges, nextFocus sql.NullString
	err := s.Scan(&r.ID, &r.WeekStart, &wins, &challenges, &nextFocus, &r.CreatedAt, &r.UpdatedAt, &r.OwnerID)
	if errors.Is(err, sql.ErrNoRows) {
		return WeekReview{}, ErrNotFound
	}
	if err != nil {
		return WeekReview{}, err
	}
	r.Wins = nullStr(wins)
	r.Challenges = nullStr(challenges)
	r.NextFocus = nullStr(nextFocus)
	return r, nil
}
