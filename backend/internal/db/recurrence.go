package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateForDate generates recurring task instances for the given date (YYYY-MM-DD).
// Smart dedup: if a non-done, non-customised instance already exists for this template,
// its planned_date is moved forward instead of creating a duplicate.
func (s *TaskStore) GenerateForDate(ctx context.Context, date string) error {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date %q: %w", date, err)
	}

	templates, err := s.ListRecurringTemplates(ctx)
	if err != nil {
		return err
	}

	for _, tmpl := range templates {
		if tmpl.RecurrenceRule == nil {
			continue
		}
		if !isDueOn(*tmpl.RecurrenceRule, t) {
			continue
		}

		pending, err := s.FindPendingRecurringInstance(ctx, tmpl.ID)
		if err != nil {
			return err
		}

		if pending != nil {
			// Carry the existing instance forward to today
			pending.PlannedDate = &date
			if _, err := s.Update(ctx, *pending); err != nil {
				return err
			}
		} else {
			// Create a fresh instance
			status := "planned"
			if _, err := s.Create(ctx, CreateTaskParams{
				ID:                  uuid.New().String(),
				Title:               tmpl.Title,
				Description:         tmpl.Description,
				PlannedDate:         &date,
				Status:              status,
				Position:            float64(time.Now().UnixMilli()),
				Tags:                tmpl.Tags,
				RecurrenceOriginID:  &tmpl.ID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// isDueOn returns true if the recurrence rule fires on the given date.
//
// Supported rules:
//
//	"daily"        – every day
//	"weekdays"     – Mon–Fri
//	"weekends"     – Sat–Sun
//	"weekly:N"     – every week on weekday N (0=Sun…6=Sat)
//	"weekly:N,N,…" – multiple weekdays
//	"monthly:D"    – day D of each month (1–31; capped to last day)
func isDueOn(rule string, t time.Time) bool {
	switch {
	case rule == "daily":
		return true

	case rule == "weekdays":
		wd := t.Weekday()
		return wd >= time.Monday && wd <= time.Friday

	case rule == "weekends":
		wd := t.Weekday()
		return wd == time.Saturday || wd == time.Sunday

	case strings.HasPrefix(rule, "weekly:"):
		days := strings.Split(strings.TrimPrefix(rule, "weekly:"), ",")
		wd := int(t.Weekday())
		for _, d := range days {
			n, err := strconv.Atoi(strings.TrimSpace(d))
			if err == nil && n == wd {
				return true
			}
		}
		return false

	case strings.HasPrefix(rule, "monthly:"):
		dayStr := strings.TrimPrefix(rule, "monthly:")
		n, err := strconv.Atoi(strings.TrimSpace(dayStr))
		if err != nil {
			return false
		}
		// Cap to last day of month
		lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
		target := n
		if target > lastDay {
			target = lastDay
		}
		return t.Day() == target
	}
	return false
}
