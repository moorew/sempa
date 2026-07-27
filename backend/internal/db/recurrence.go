package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateForDate runs the daily "smart rollover" for `date` (YYYY-MM-DD) and
// makes sure every template due that day has a fresh instance.
//
// Smart rollover — applied to every recurring instance still open *before* `date`:
//
//	pristine  (untouched: not customised, not started, no logged time)
//	          → deleted; today's fresh instance takes its place (no pile-up).
//	modified  (customised, in-progress, or with logged time)
//	          → carried forward to `date` (and its week_start realigned), so the
//	            user keeps their notes/subtasks and sees it alongside today's new
//	            instance.
//
// Pristine vs. modified is tracked by tasks.is_customized (set whenever the user
// edits an instance's content or adds a sub-task — see the API layer).
func (s *TaskStore) GenerateForDate(ctx context.Context, date string) error {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date %q: %w", date, err)
	}
	ws := weekStartOf(t)

	// 1a. Delete pristine, open instances left in the past — nothing the user
	//     cared about, so today's fresh instance replaces them.
	if err := s.tombstoneAndDeleteTasks(ctx, `
		recurrence_origin_id IS NOT NULL
		  AND is_customized = 0
		  AND status IN ('backlog','planned')
		  AND (time_actual_minutes IS NULL OR time_actual_minutes = 0)
		  AND planned_date IS NOT NULL
		  AND planned_date < ?`, date); err != nil {
		return err
	}

	// 1b. Whatever past, open recurring instances remain are "modified" — carry
	//     them forward to today and realign week_start so ListByWeek finds them.
	if _, err := s.db.ExecContext(ctx, `
		UPDATE tasks
		SET planned_date = ?, week_start = ?, updated_at = datetime('now')
		WHERE recurrence_origin_id IS NOT NULL
		  AND status IN ('backlog','planned','in_progress')
		  AND planned_date IS NOT NULL
		  AND planned_date < ?`, date, ws, date); err != nil {
		return err
	}

	// 1c. Heal any same-day duplicates: keep one instance per template per day,
	//     preferring the customised / time-logged / done one over a bare pristine
	//     copy. (Earlier logic could leave both a carried-forward modified instance
	//     and a fresh pristine one on the same day.)
	if err := s.dedupeInstancesForDate(ctx, date); err != nil {
		return err
	}

	// 2. Ensure ONE instance exists for every template due on `date`. Any
	//    non-cancelled instance already on the day counts — a carried-forward or
	//    customised instance IS that day's occurrence, so we don't add a duplicate.
	return s.ensureInstancesForDate(ctx, t)
}

// ensureInstancesForDate is the non-destructive half of rollover: it creates a
// pristine instance for every template due on `t` that doesn't already have one.
// It never deletes or moves anything, so it is safe to run from the timezone-
// agnostic horizon poller (which must not delete a still-current instance based
// on the server's own midnight — see SeedHorizon).
func (s *TaskStore) ensureInstancesForDate(ctx context.Context, t time.Time) error {
	date := t.Format("2006-01-02")
	templates, err := s.ListRecurringTemplates(ctx, SystemScope)
	if err != nil {
		return err
	}
	for _, tmpl := range templates {
		if tmpl.RecurrenceRule == nil || !isDueOn(*tmpl.RecurrenceRule, t) {
			continue
		}
		if s.instanceExistsForDate(ctx, tmpl.ID, date) {
			continue
		}
		if err := s.createInstance(ctx, tmpl, t); err != nil {
			return err
		}
	}
	return nil
}

// GenerateForDay backs the by-date list endpoint. Crucially, destructive rollover
// (delete pristine past, carry forward modified) is anchored to the caller's real
// `today` — NOT to the date being viewed — so opening a future day never deletes
// today's instances, and a travelling client rolls the day over on its own local
// midnight. The viewed date's instances are then ensured non-destructively.
// `today` is the client's device date (YYYY-MM-DD); "" falls back to the server's.
func (s *TaskStore) GenerateForDay(ctx context.Context, date, today string) error {
	if today == "" {
		today = ServerToday()
	}
	if err := s.GenerateForDate(ctx, today); err != nil {
		return err
	}
	// For a future day, also materialise its instances. Past days were already
	// settled by the rollover above, so we leave them alone.
	if date > today {
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			return fmt.Errorf("invalid date %q: %w", date, err)
		}
		return s.ensureInstancesForDate(ctx, t)
	}
	return nil
}

// dedupeInstancesForDate heals same-day duplicates for a single day.
func (s *TaskStore) dedupeInstancesForDate(ctx context.Context, date string) error {
	return s.dedupeRecurringInstances(ctx, date)
}

// dedupeRecurringInstances removes redundant pristine duplicates of a recurring
// template on a day, keeping the single most meaningful instance. Pass a specific
// `date` to scope the heal to one day, or "" to heal every day at once.
//
// Only *pristine* instances (not customised, no logged time) are ever deletion
// candidates, so nothing the user actually touched — notes, sub-tasks, tracked
// time — is lost. A candidate is dropped only when a same-day sibling ranks
// higher, so a day is never left without an instance, and a still-current sole
// instance is never removed (making this safe on the timezone-agnostic poller).
//
// Instances are ranked so the survivor is the most-progressed one, and among
// equals the earliest-created (then lowest id) wins — fully deterministic, so
// two seeders/pollers racing agree on which copy to keep:
//
//	4  customised OR time logged   (never a candidate — protected outright)
//	3  done
//	2  in_progress
//	1  planned / backlog
//
// A done pristine instance therefore survives against a merely-planned sibling
// (a completed day is never downgraded), while two identical pristine done copies
// collapse to one — the exact-same-created_at pair a pre-fix seed race left in a
// user's history, double-counting the completed day.
//
// The all-days form is what catches the cross-week duplicate: a race between the
// horizon poller and concurrent client week-fetches can seed two pristine
// instances on a FUTURE day, and — because per-day rollover only ever deduped
// "today" — that pair used to survive across the week boundary until the day
// finally became today. Deduping every seeded day closes that gap.
func (s *TaskStore) dedupeRecurringInstances(ctx context.Context, date string) error {
	// rank(t) mirrors the doc comment; identical expression for candidate and
	// sibling so the comparison is symmetric.
	const rank = `CASE
		WHEN %[1]s.is_customized = 1
		  OR (%[1]s.time_actual_minutes IS NOT NULL AND %[1]s.time_actual_minutes > 0) THEN 4
		WHEN %[1]s.status = 'done'        THEN 3
		WHEN %[1]s.status = 'in_progress' THEN 2
		ELSE 1 END`
	candRank := fmt.Sprintf(rank, "tasks")
	sibRank := fmt.Sprintf(rank, "o")
	where := `
		recurrence_origin_id IS NOT NULL
		  AND is_customized = 0
		  AND (time_actual_minutes IS NULL OR time_actual_minutes = 0)
		  AND status IN ('backlog','planned','done')
		  AND EXISTS (
			SELECT 1 FROM tasks o
			WHERE o.recurrence_origin_id = tasks.recurrence_origin_id
			  AND o.planned_date = tasks.planned_date
			  AND o.id <> tasks.id
			  AND o.status <> 'cancelled'
			  AND (
				` + sibRank + ` > ` + candRank + `
				OR (` + sibRank + ` = ` + candRank + ` AND o.created_at < tasks.created_at)
				OR (` + sibRank + ` = ` + candRank + ` AND o.created_at = tasks.created_at AND o.id < tasks.id)
			  )
		  )`
	if date != "" {
		return s.tombstoneAndDeleteTasks(ctx, where+` AND planned_date = ?`, date)
	}
	return s.tombstoneAndDeleteTasks(ctx, where)
}

// tombstoneAndDeleteTasks deletes the tasks matching `where` AND records a sync
// tombstone for each, so offline/local-first clients learn to drop them. Without
// this, recurrence deletes (which bypass the normal delete handler) silently
// stranded stale instances on devices — the cause of phantom duplicate
// recurring tasks. `where` references the `tasks` table (no alias needed).
func (s *TaskStore) tombstoneAndDeleteTasks(ctx context.Context, where string, args ...any) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO sync_tombstones (entity_type, entity_id, deleted_at, owner_id, was_shared, kind)
		 SELECT 'task', id, datetime('now'), owner_id, shared, 'delete' FROM tasks WHERE `+where, args...); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE `+where, args...)
	return err
}

// GenerateForWeek ensures the requested week has the right recurring instances.
// `today` is the caller's local date (YYYY-MM-DD); pass "" to use the server's.
// Passing the client's date keeps rollover correct across timezones.
//
// This is intentionally non-destructive: it never deletes future instances (an
// earlier version did, which could race with the day/today view and make tasks
// vanish). Pristine-existence checks keep it idempotent and duplicate-free.
func (s *TaskStore) GenerateForWeek(ctx context.Context, weekStart, today string) error {
	if today == "" {
		today = ServerToday()
	}
	ws, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return fmt.Errorf("invalid weekStart %q: %w", weekStart, err)
	}
	weekEnd := ws.AddDate(0, 0, 6).Format("2006-01-02")

	// Backfill: repair any task whose week_start is missing or doesn't match its
	// planned_date, so it surfaces in ListByWeek (which filters on week_start).
	s.db.ExecContext(ctx, `
		UPDATE tasks
		SET week_start = date(planned_date, '-' || ((CAST(strftime('%w', planned_date) AS INTEGER) + 6) % 7) || ' days')
		WHERE planned_date IS NOT NULL
		  AND (week_start IS NULL OR week_start = ''
		       OR week_start != date(planned_date, '-' || ((CAST(strftime('%w', planned_date) AS INTEGER) + 6) % 7) || ' days'))`)

	switch {
	case today > weekEnd:
		// Past week — instances are settled; nothing to do.
		return nil
	case today < weekStart:
		// Future week — seed every due day, no rollover.
		return s.seedWeekInstances(ctx, ws, "")
	default:
		// Current week — roll today over, then seed the remaining days.
		if err := s.GenerateForDate(ctx, today); err != nil {
			return err
		}
		return s.seedWeekInstances(ctx, ws, today)
	}
}

// GenerateHorizon ensures recurring instances exist for the current week and
// the next `weeksAhead` weeks. Offline-first clients (Tauri desktop, Capacitor
// Android) read tasks straight from their local SQLite DB and never hit the
// HTTP list endpoint that lazily generates instances, so without this the
// series appears to "end" at the last week a web client happened to request.
// Run proactively by the recurrence poller, these instances flow to every
// client through normal sync. `today` is YYYY-MM-DD; pass "" for the server's
// date. It is idempotent (pristine-existence checks keep it duplicate-free).
func (s *TaskStore) GenerateHorizon(ctx context.Context, today string, weeksAhead int) error {
	if today == "" {
		today = ServerToday()
	}
	t, err := time.Parse("2006-01-02", today)
	if err != nil {
		return fmt.Errorf("invalid today %q: %w", today, err)
	}
	curWS, err := time.Parse("2006-01-02", weekStartOf(t))
	if err != nil {
		return err
	}
	for i := 0; i <= weeksAhead; i++ {
		ws := curWS.AddDate(0, 0, i*7).Format("2006-01-02")
		if err := s.GenerateForWeek(ctx, ws, today); err != nil {
			return err
		}
	}
	return nil
}

// SeedHorizon is the recurrence poller's entry point: it materialises future
// instances for offline-first clients WITHOUT the destructive rollover that
// GenerateForDate/GenerateForWeek perform. This matters because the poller fires
// on a server timer with no client context — if it deleted "past pristine"
// instances at the server's own midnight, a user travelling west of the home
// zone would lose a still-current task hours early (the very bug that started in
// UTC). Instead:
//
//   - Exact same-day rollover is left to active clients, which call
//     GenerateForWeek/GenerateForDay with their own device date.
//   - The poller only ENSURES upcoming instances exist, plus a conservative
//     cleanup of pristine instances older than a 2-day grace window. 2 days
//     safely exceeds the ~26h maximum spread between any two timezones, so the
//     timer can never delete something that is still "today" for the user.
//
// `today` is YYYY-MM-DD in the server's home zone; "" uses ServerToday().
func (s *TaskStore) SeedHorizon(ctx context.Context, today string, weeksAhead int) error {
	if today == "" {
		today = ServerToday()
	}
	t, err := time.Parse("2006-01-02", today)
	if err != nil {
		return fmt.Errorf("invalid today %q: %w", today, err)
	}

	// Bound pile-up of abandoned pristine instances without risking the current
	// day in any timezone (2-day grace > max inter-zone spread).
	grace := t.AddDate(0, 0, -2).Format("2006-01-02")
	if err := s.tombstoneAndDeleteTasks(ctx, `
		recurrence_origin_id IS NOT NULL
		  AND is_customized = 0
		  AND status IN ('backlog','planned')
		  AND (time_actual_minutes IS NULL OR time_actual_minutes = 0)
		  AND planned_date IS NOT NULL
		  AND planned_date < ?`, grace); err != nil {
		return err
	}

	curWS, err := time.Parse("2006-01-02", weekStartOf(t))
	if err != nil {
		return err
	}
	for i := 0; i <= weeksAhead; i++ {
		ws := curWS.AddDate(0, 0, i*7)
		// Current week: seed today onward (afterDate = yesterday). Future weeks:
		// seed every due day.
		after := ""
		if i == 0 {
			after = t.AddDate(0, 0, -1).Format("2006-01-02")
		}
		if err := s.seedWeekInstances(ctx, ws, after); err != nil {
			return err
		}
	}

	// Global reconcile: collapse any same-day duplicate recurring instances across
	// ALL dates, not just the seeded horizon. seedWeekInstances only heals the
	// current + upcoming weeks, so a duplicate a pre-fix race left on a PAST day
	// (e.g. two identical done copies double-counting a completed day) would never
	// be reached otherwise. Safe on the timezone-agnostic poller: it only removes
	// a pristine instance shadowed by a higher-ranked same-day sibling, never a
	// day's sole instance.
	return s.dedupeRecurringInstances(ctx, "")
}

// seedWeekInstances creates a pristine instance for each due day in the week
// (Mon–Sun) strictly after `afterDate` (use "" to include all days). Days that
// already have a pristine instance for the template are skipped.
func (s *TaskStore) seedWeekInstances(ctx context.Context, ws time.Time, afterDate string) error {
	templates, err := s.ListRecurringTemplates(ctx, SystemScope)
	if err != nil {
		return err
	}
	for i := 0; i < 7; i++ {
		d := ws.AddDate(0, 0, i)
		date := d.Format("2006-01-02")
		if afterDate != "" && date <= afterDate {
			continue
		}
		for _, tmpl := range templates {
			if tmpl.RecurrenceRule == nil || !isDueOn(*tmpl.RecurrenceRule, d) {
				continue
			}
			if s.instanceExistsForDate(ctx, tmpl.ID, date) {
				continue
			}
			if err := s.createInstance(ctx, tmpl, d); err != nil {
				return err
			}
		}
		// Heal any duplicate a concurrent seeder (another client week-fetch or the
		// horizon poller) raced us to create on this day. instanceExistsForDate is
		// a non-atomic check with no unique constraint behind it, so two seeders
		// can both miss and both insert; deduping here — on every seeded day, not
		// just today — stops a future-day duplicate from riding across the week.
		if err := s.dedupeRecurringInstances(ctx, date); err != nil {
			return err
		}
	}
	return nil
}

// createInstance materialises one recurring instance for the given template on
// day `t`, copying the template's roughly_at sort hint.
func (s *TaskStore) createInstance(ctx context.Context, tmpl Task, t time.Time) error {
	date := t.Format("2006-01-02")
	ws := weekStartOf(t)
	_, err := s.Create(ctx, CreateTaskParams{
		ID:                  uuid.New().String(),
		Title:               tmpl.Title,
		Description:         tmpl.Description,
		PlannedDate:         &date,
		WeekStart:           &ws,
		Status:              "planned",
		Position:            float64(t.UnixMilli()),
		TimeEstimateMinutes: tmpl.TimeEstimateMinutes,
		Tags:                tmpl.Tags,
		RecurrenceOriginID:  &tmpl.ID,
		RoughlyAt:           tmpl.RoughlyAt,
		// Instances inherit the template's ownership + share state.
		OwnerID: tmpl.OwnerID,
		Shared:  tmpl.Shared,
	})
	return err
}

// RetireTemplate ends a recurring series WITHOUT deleting the template row.
//
// Why not just DELETE it: `recurrence_origin_id` is declared
// `REFERENCES tasks(id) ON DELETE SET NULL`, so removing the template row makes
// SQLite silently NULL the origin on every instance ever generated. That is
// catastrophic and completely invisible:
//
//   - Generation stops (ListRecurringTemplates has nothing to seed from), so the
//     series quietly disappears once the last pre-seeded instance rolls past.
//   - Every rollover and dedup query keys on `recurrence_origin_id IS NOT NULL`,
//     so the detached instances become permanently unmanageable — any duplicate
//     pair already on a day freezes there forever.
//   - No tombstones are written for the detached rows, so nothing propagates and
//     nothing can be recovered.
//
// This is not hypothetical: it is exactly how a user's daily template was lost,
// taking ~40 instances with it, from a single stray DELETE.
//
// Retiring instead flips the template to 'cancelled' and keeps the row. The FK
// never fires, instances keep their link (so rollover/dedup keep working on
// history), generation stops because ListRecurringTemplates filters cancelled,
// and the series stays recoverable. Open, untouched instances are removed with
// proper tombstones so no ghost cards are left on the board.
//
// Deliberately a store method called from the delete handler, NOT an update: a
// template PATCH triggers SyncTemplateInstances, which would delete and instantly
// regenerate the whole horizon.
func (s *TaskStore) RetireTemplate(ctx context.Context, id string) error {
	// Drop the open, untouched instances — they're just placeholders for days the
	// user no longer wants. Customised / in-progress / time-logged / done ones are
	// the user's actual work and stay as history, still linked to the template.
	if err := s.tombstoneAndDeleteTasks(ctx, `
		recurrence_origin_id = ?
		  AND is_customized = 0
		  AND status IN ('backlog','planned')
		  AND (time_actual_minutes IS NULL OR time_actual_minutes = 0)`, id); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status = 'cancelled', updated_at = datetime('now')
		 WHERE id = ? AND recurrence_rule IS NOT NULL AND recurrence_origin_id IS NULL`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SyncTemplateInstances propagates a template's current content to its future,
// untouched instances. Pristine (non-customised, open, no logged time) instances
// from `today` onward are deleted and regenerated under the template's current
// title / description / tags / estimate / rule. Customised or already-worked
// instances are left exactly as they are.
func (s *TaskStore) SyncTemplateInstances(ctx context.Context, originID, today string) error {
	if today == "" {
		today = ServerToday()
	}
	if err := s.tombstoneAndDeleteTasks(ctx, `
		recurrence_origin_id = ?
		  AND planned_date >= ?
		  AND is_customized = 0
		  AND status IN ('backlog','planned')
		  AND (time_actual_minutes IS NULL OR time_actual_minutes = 0)`, originID, today); err != nil {
		return err
	}
	return s.GenerateHorizon(ctx, today, recurrenceHorizonWeeks)
}

// recurrenceHorizonWeeks mirrors the poller's horizon (+2 weeks).
const recurrenceHorizonWeeks = 2

// RecurringInstanceRef is a lightweight (id, origin, date) tuple for one
// recurring instance — the authoritative set a client reconciles against.
type RecurringInstanceRef struct {
	ID     string `json:"id"`
	Origin string `json:"origin"`
	Date   string `json:"date"`
}

// RecurringInstanceIndex returns every current (non-cancelled, dated) recurring
// instance. Clients use this as the source of truth to drop locally-stranded
// instances the server no longer has (orphans from pre-tombstone deletes).
func (s *TaskStore) RecurringInstanceIndex(ctx context.Context, ownerID string) ([]RecurringInstanceRef, error) {
	scope, sargs := visScope(ownerID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, recurrence_origin_id, planned_date FROM tasks
		WHERE recurrence_origin_id IS NOT NULL
		  AND status != 'cancelled'
		  AND planned_date IS NOT NULL`+scope, sargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RecurringInstanceRef{}
	for rows.Next() {
		var ref RecurringInstanceRef
		if err := rows.Scan(&ref.ID, &ref.Origin, &ref.Date); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// instanceExistsForDate reports whether ANY non-cancelled instance of the
// template already exists on the given date — customised or pristine. A
// customised (or carried-forward) instance IS that day's occurrence, so it
// suppresses creating a duplicate: one instance per template per day.
func (s *TaskStore) instanceExistsForDate(ctx context.Context, originID, date string) bool {
	var count int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks
		 WHERE recurrence_origin_id = ? AND planned_date = ?
		   AND status != 'cancelled'`,
		originID, date).Scan(&count)
	return count > 0
}

// weekStartOf returns the Monday-based week-start date (YYYY-MM-DD) for t,
// matching the frontend's weekStart() convention so recurring instances are
// found by ListByWeek (which filters on week_start).
func weekStartOf(t time.Time) string {
	offset := int(t.Weekday()) - int(time.Monday) // Sun=0 → -1, Mon=1 → 0, …
	if offset < 0 {
		offset += 7
	}
	return t.AddDate(0, 0, -offset).Format("2006-01-02")
}

// isDueOn reports whether the recurrence rule fires on date t.
//
// Supported rules:
//
//	"daily"          – every day
//	"weekdays"       – Mon–Fri
//	"weekends"       – Sat–Sun
//	"weekly:N"       – weekday N (0=Sun … 6=Sat)
//	"weekly:N,N,…"   – multiple weekdays
//	"monthly:D"      – day D of each month (capped to last day)
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
			if n, err := strconv.Atoi(strings.TrimSpace(d)); err == nil && n == wd {
				return true
			}
		}
		return false
	case strings.HasPrefix(rule, "monthly:"):
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(rule, "monthly:")))
		if err != nil {
			return false
		}
		lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
		if n > lastDay {
			n = lastDay
		}
		return t.Day() == n
	}
	return false
}
