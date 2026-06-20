package db

import (
	"context"
	"database/sql"
	"errors"
)

type DailyPlan struct {
	ID         string  `json:"id"`
	PlanDate   string  `json:"plan_date"`
	Status     string  `json:"status"`
	Intention  *string `json:"intention"`
	Reflection *string `json:"reflection"`
	Wins       *string `json:"wins"`
	ShutdownAt *string `json:"shutdown_at"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	OwnerID    string  `json:"owner_id"`
}

const planCols = `id, plan_date, status, intention, reflection, wins, shutdown_at, created_at, updated_at, owner_id`

func scanPlan(s scanner) (DailyPlan, error) {
	var p DailyPlan
	var intention, reflection, wins, shutdownAt sql.NullString
	err := s.Scan(&p.ID, &p.PlanDate, &p.Status, &intention, &reflection, &wins, &shutdownAt, &p.CreatedAt, &p.UpdatedAt, &p.OwnerID)
	if err != nil {
		return DailyPlan{}, err
	}
	p.Intention = nullStr(intention)
	p.Reflection = nullStr(reflection)
	p.Wins = nullStr(wins)
	p.ShutdownAt = nullStr(shutdownAt)
	return p, nil
}

type DailyPlanStore struct{ db *sql.DB }

func NewDailyPlanStore(db *sql.DB) *DailyPlanStore { return &DailyPlanStore{db: db} }

func (s *DailyPlanStore) Get(ctx context.Context, date, ownerID string) (DailyPlan, error) {
	scope, sargs := ownScope(ownerID)
	row := s.db.QueryRowContext(ctx, `SELECT `+planCols+` FROM daily_plans WHERE plan_date = ?`+scope,
		append([]any{date}, sargs...)...)
	p, err := scanPlan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return DailyPlan{}, ErrNotFound
	}
	return p, err
}

// List returns daily plans that hold something worth journalling (a non-empty
// intention or reflection), newest first. limit <= 0 means no limit.
func (s *DailyPlanStore) List(ctx context.Context, limit int, ownerID string) ([]DailyPlan, error) {
	scope, sargs := ownScope(ownerID)
	q := `SELECT ` + planCols + ` FROM daily_plans
		 WHERE ((intention IS NOT NULL AND intention != '')
		    OR (reflection IS NOT NULL AND reflection != ''))` + scope + `
		 ORDER BY plan_date DESC`
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
	plans := []DailyPlan{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

type UpsertPlanParams struct {
	ID         string
	PlanDate   string
	Status     string
	Intention  *string
	Reflection *string
	Wins       *string
	ShutdownAt *string
	OwnerID    string
}

func (s *DailyPlanStore) Upsert(ctx context.Context, p UpsertPlanParams) (DailyPlan, error) {
	owner, err := resolveOwner(ctx, s.db, p.OwnerID)
	if err != nil {
		return DailyPlan{}, err
	}
	// Conflict is per (owner, date): each user has at most one plan per day.
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO daily_plans (id, plan_date, status, intention, reflection, wins, shutdown_at, owner_id)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(owner_id, plan_date) DO UPDATE SET
			status      = excluded.status,
			intention   = excluded.intention,
			reflection  = excluded.reflection,
			wins        = excluded.wins,
			shutdown_at = excluded.shutdown_at,
			updated_at  = datetime('now')
		RETURNING `+planCols,
		p.ID, p.PlanDate, p.Status, p.Intention, p.Reflection, p.Wins, p.ShutdownAt, owner,
	)
	return scanPlan(row)
}
