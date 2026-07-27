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

// DurationSample is one completed task's logged duration. Unlike TimeSample it
// carries no estimate, because it answers a different question: "how long does
// this KIND of work actually take?" rather than "how far off are my estimates?".
type DurationSample struct {
	Title   string   `json:"title"`
	Minutes int64    `json:"minutes"`
	Tags    []string `json:"tags"`
}

// CompletedDurationSamples returns recent completed tasks with logged time,
// whether or not they were ever estimated.
//
// Deliberately NOT reusing CompletedTimeSamples: that one requires
// `time_estimate_minutes > 0` because a ratio needs a denominator. Duration
// learning doesn't, and applying the same filter here would silently discard
// every task the user completed without estimating first — shrinking the pool and
// biasing it toward the tasks they happened to plan carefully.
func (s *TaskStore) CompletedDurationSamples(ctx context.Context, limit int, ownerID string) ([]DurationSample, error) {
	if limit <= 0 {
		limit = 500
	}
	// Personal calibration of your own work — owner-only, not owner-or-shared.
	scope, sargs := ownScope(ownerID)
	args := append([]any{}, sargs...)
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT title, time_actual_minutes, tags
		FROM tasks
		WHERE status = 'done' AND archived_at IS NULL
		  AND time_actual_minutes > 0`+scope+`
		ORDER BY completed_at DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DurationSample{}
	for rows.Next() {
		var title string
		var mins int64
		var tagsJSON string
		if err := rows.Scan(&title, &mins, &tagsJSON); err != nil {
			return nil, err
		}
		tags := []string{}
		if tagsJSON != "" {
			_ = json.Unmarshal([]byte(tagsJSON), &tags)
		}
		out = append(out, DurationSample{Title: title, Minutes: mins, Tags: tags})
	}
	return out, rows.Err()
}

// CompletedTimeSamples returns recent completed tasks that carry both an
// estimate and a logged actual — the raw material for the time-blindness
// profile. Ordered newest-first and capped so the stats reflect recent habits.
func (s *TaskStore) CompletedTimeSamples(ctx context.Context, limit int, ownerID string) ([]TimeSample, error) {
	if limit <= 0 {
		limit = 1000
	}
	// Insights are a personal calibration of *your own* completed work, so this is
	// owner-only (not owner-or-shared).
	scope, sargs := ownScope(ownerID)
	args := append([]any{}, sargs...)
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT title, time_estimate_minutes, time_actual_minutes, tags
		FROM tasks
		WHERE status = 'done' AND archived_at IS NULL
		  AND time_estimate_minutes > 0 AND time_actual_minutes > 0`+scope+`
		ORDER BY completed_at DESC
		LIMIT ?`, args...)
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
