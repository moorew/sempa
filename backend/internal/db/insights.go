package db

import (
	"context"
	"encoding/json"
)

// TimeSample is one completed task's estimate-vs-actual data point, used to
// build the planned-vs-actual profile that powers calibrated time predictions.
type TimeSample struct {
	Title    string
	Estimate int64
	Actual   int64
	Tags     []string
}

// CompletedTimeSamples returns recent completed tasks that carry both an
// estimate and a logged actual — the raw material for the time-blindness
// profile. Ordered newest-first and capped so the stats reflect recent habits.
func (s *TaskStore) CompletedTimeSamples(ctx context.Context, limit int) ([]TimeSample, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT title, time_estimate_minutes, time_actual_minutes, tags
		FROM tasks
		WHERE status = 'done' AND archived_at IS NULL
		  AND time_estimate_minutes > 0 AND time_actual_minutes > 0
		ORDER BY completed_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimeSample
	for rows.Next() {
		var title string
		var est, act int64
		var tagsJSON string
		if err := rows.Scan(&title, &est, &act, &tagsJSON); err != nil {
			return nil, err
		}
		var tags []string
		if tagsJSON != "" {
			_ = json.Unmarshal([]byte(tagsJSON), &tags)
		}
		out = append(out, TimeSample{Title: title, Estimate: est, Actual: act, Tags: tags})
	}
	return out, rows.Err()
}
