package db

import (
	"context"
	"fmt"
	"strings"
)

// buildTagFilter returns an SQL fragment (and its args) that constrains a JSON
// `tags` array column. matchAll=false → row has ANY of the tags (OR);
// matchAll=true → row has ALL of them (AND). The IIF guards rows whose tags are
// NULL/'' so json_each never errors.
func buildTagFilter(tags []string, matchAll bool) (string, []any) {
	if len(tags) == 0 {
		return "", nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(tags)), ",")
	args := make([]any, 0, len(tags)+1)
	for _, t := range tags {
		args = append(args, t)
	}
	src := "json_each(IIF(tags IS NULL OR tags = '', '[]', tags))"
	if matchAll {
		args = append(args, len(tags))
		return fmt.Sprintf("(SELECT COUNT(DISTINCT value) FROM %s WHERE value IN (%s)) = ?", src, ph), args
	}
	return fmt.Sprintf("EXISTS (SELECT 1 FROM %s WHERE value IN (%s))", src, ph), args
}

// Search returns tasks matching a free-text query (title/description) and/or a
// set of tags. Empty q + empty tags returns nothing. Excludes sub-tasks.
func (s *TaskStore) Search(ctx context.Context, q string, tags []string, matchAll bool, limit int) ([]Task, error) {
	where := []string{"parent_task_id IS NULL"}
	var args []any

	if q != "" {
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(COALESCE(description,'')) LIKE ?)")
		like := "%" + strings.ToLower(q) + "%"
		args = append(args, like, like)
	}
	if clause, tagArgs := buildTagFilter(tags, matchAll); clause != "" {
		where = append(where, clause)
		args = append(args, tagArgs...)
	}
	if q == "" && len(tags) == 0 {
		return []Task{}, nil
	}

	query := `SELECT ` + taskCols + ` FROM tasks WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY COALESCE(planned_date, date(updated_at)) DESC, updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Search returns objectives whose title/description match the query.
func (s *ObjectiveStore) Search(ctx context.Context, q string, limit int) ([]Objective, error) {
	if q == "" {
		return []Objective{}, nil
	}
	like := "%" + strings.ToLower(q) + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+objCols+` FROM objectives
		 WHERE LOWER(title) LIKE ? OR LOWER(COALESCE(description,'')) LIKE ?
		 ORDER BY week_start DESC LIMIT ?`, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Objective{}
	for rows.Next() {
		o, err := scanObjective(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Search returns daily plans whose intention/reflection/wins match the query.
func (s *DailyPlanStore) Search(ctx context.Context, q string, limit int) ([]DailyPlan, error) {
	if q == "" {
		return []DailyPlan{}, nil
	}
	like := "%" + strings.ToLower(q) + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+planCols+` FROM daily_plans
		 WHERE LOWER(COALESCE(intention,'')) LIKE ? OR LOWER(COALESCE(reflection,'')) LIKE ?
		    OR LOWER(COALESCE(wins,'')) LIKE ?
		 ORDER BY plan_date DESC LIMIT ?`, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DailyPlan{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Search returns week reviews whose wins/challenges/next_focus match the query.
func (s *WeekReviewStore) Search(ctx context.Context, q string, limit int) ([]WeekReview, error) {
	if q == "" {
		return []WeekReview{}, nil
	}
	like := "%" + strings.ToLower(q) + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, week_start, wins, challenges, next_focus, created_at, updated_at
		 FROM week_reviews
		 WHERE LOWER(COALESCE(wins,'')) LIKE ? OR LOWER(COALESCE(challenges,'')) LIKE ?
		    OR LOWER(COALESCE(next_focus,'')) LIKE ?
		 ORDER BY week_start DESC LIMIT ?`, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WeekReview{}
	for rows.Next() {
		var wr WeekReview
		if err := rows.Scan(&wr.ID, &wr.WeekStart, &wr.Wins, &wr.Challenges, &wr.NextFocus,
			&wr.CreatedAt, &wr.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, wr)
	}
	return out, rows.Err()
}
