// Package calsync centralises read-only calendar syncing (ICS subscriptions and
// the Fastmail/CalDAV calendar) so the HTTP handlers and the background poller
// share one implementation and never drift in how events are parsed/stored.
package calsync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/caldav"
	"github.com/clevercode/sempa/internal/integrations/ical"
)

// SyncICalSubscription fetches one ICS feed and upserts its events. On a fetch
// error the subscription's error_msg is recorded (and returned) so the UI can
// surface it; the stored events are left untouched.
func SyncICalSubscription(ctx context.Context, store *db.ICalStore, sub db.ICalSubscription) error {
	events, err := ical.Fetch(sub.URL)
	if err != nil {
		store.SetError(ctx, sub.ID, err.Error())
		return err
	}
	dbEvents := make([]db.ICalEvent, 0, len(events))
	for _, ev := range events {
		if ev.UID == "" || ev.StartTime == "" {
			continue
		}
		dbEvents = append(dbEvents, db.ICalEvent{
			ID:             uuid.New().String(),
			SubscriptionID: sub.ID,
			UID:            ev.UID,
			Summary:        ev.Summary,
			Description:    ev.Description,
			Location:       ev.Location,
			URL:            ev.URL,
			StartTime:      ev.StartTime,
			EndTime:        ev.EndTime,
			AllDay:         ev.AllDay,
		})
	}
	return store.UpsertEvents(ctx, sub.ID, dbEvents)
}

// SyncAllICalSubscriptions re-syncs every ICS subscription. It returns the
// number of feeds that synced cleanly and the number that errored.
func SyncAllICalSubscriptions(ctx context.Context, database *sql.DB) (ok int, failed int) {
	store := db.NewICalStore(database)
	subs, err := store.ListSubscriptions(ctx)
	if err != nil {
		return 0, 0
	}
	for _, sub := range subs {
		if err := SyncICalSubscription(ctx, store, sub); err != nil {
			failed++
		} else {
			ok++
		}
	}
	return ok, failed
}

// ErrFastmailCalendarDisabled means the Fastmail calendar integration is not
// connected/enabled, so there is nothing to sync (a no-op, not a failure).
var ErrFastmailCalendarDisabled = errors.New("fastmail calendar not enabled")

// SyncFastmailCalendar re-reads the Fastmail calendar over CalDAV and replaces
// the stored snapshot. Returns the event count and the synced date window.
func SyncFastmailCalendar(ctx context.Context, database *sql.DB) (count int, from, to string, err error) {
	configs := db.NewIntegrationConfigStore(database)

	// Only sync when the user has enabled the Fastmail calendar.
	if cfg, gerr := configs.Get(ctx, "fastmail_calendar"); gerr != nil || !cfg.Enabled {
		return 0, "", "", ErrFastmailCalendarDisabled
	}

	// CalDAV reuses the stored Fastmail app-password credentials (JMAP rejects
	// them, CalDAV accepts them).
	fmCfg, gerr := configs.Get(ctx, "fastmail")
	if gerr != nil {
		return 0, "", "", ErrFastmailCalendarDisabled
	}
	var fm struct {
		Email       string `json:"email"`
		AppPassword string `json:"app_password"`
	}
	if jerr := json.Unmarshal([]byte(fmCfg.Config), &fm); jerr != nil {
		return 0, "", "", errors.New("malformed fastmail config")
	}

	client, cerr := caldav.NewClient(caldav.Config{
		BaseURL:  caldav.FastmailBaseURL,
		Username: fm.Email,
		Password: fm.AppPassword,
	})
	if cerr != nil {
		return 0, "", "", cerr
	}

	// Sync 4 weeks: 1 in the past + 3 ahead (matches the manual sync window).
	now := time.Now()
	from = now.AddDate(0, 0, -7).Format("2006-01-02")
	to = now.AddDate(0, 0, 21).Format("2006-01-02")

	events, rerr := caldav.ReadCalendarEvents(ctx, client, from, to)
	if rerr != nil {
		return 0, from, to, rerr
	}

	fmStore := db.NewFastmailCalStore(database)
	// Replace the snapshot so server-side deletions/moves don't linger.
	if derr := fmStore.DeleteAll(ctx); derr != nil {
		return 0, from, to, derr
	}
	if uerr := fmStore.UpsertEvents(ctx, events); uerr != nil {
		return 0, from, to, uerr
	}
	_ = configs.TouchSyncTime(ctx, "fastmail_calendar")
	return len(events), from, to, nil
}
