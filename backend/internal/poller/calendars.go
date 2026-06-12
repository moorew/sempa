package poller

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/calsync"
)

// StartCalendars periodically re-syncs every read-only calendar source — ICS
// subscriptions and the Fastmail/CalDAV calendar — so events stay fresh (and
// timezone-correct) without the user manually hitting "Sync". It runs an
// initial pass shortly after boot, then on the given interval.
func StartCalendars(ctx context.Context, database *sql.DB, interval time.Duration) {
	if interval < 1*time.Minute {
		interval = 1 * time.Minute
	}
	go func() {
		// Small delay so startup migrations / first requests settle first.
		select {
		case <-time.After(15 * time.Second):
		case <-ctx.Done():
			return
		}
		pollCalendars(ctx, database)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pollCalendars(ctx, database)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func pollCalendars(ctx context.Context, database *sql.DB) {
	if ok, failed := calsync.SyncAllICalSubscriptions(ctx, database); ok > 0 || failed > 0 {
		slog.Info("calendar poller: ical synced", "ok", ok, "failed", failed)
	}

	count, _, _, err := calsync.SyncFastmailCalendar(ctx, database)
	switch {
	case errors.Is(err, calsync.ErrFastmailCalendarDisabled):
		// Not connected/enabled — nothing to do.
	case err != nil:
		slog.Error("calendar poller: fastmail sync failed", "err", err)
	default:
		slog.Info("calendar poller: fastmail synced", "events", count)
	}
}
