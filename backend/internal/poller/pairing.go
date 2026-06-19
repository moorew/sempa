package poller

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/db"
)

// StartPairingCleanup prunes expired, never-approved device pairings on a slow
// interval so the public /devices/pair/start endpoint can't grow the table
// without bound. Idempotent; safe to run alongside everything else.
func StartPairingCleanup(ctx context.Context, database *sql.DB) {
	store := db.NewPairingStore(database)
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			if n, err := store.PurgeExpired(); err != nil {
				slog.Warn("pairing cleanup", "err", err)
			} else if n > 0 {
				slog.Info("pairing cleanup", "purged", n)
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()
}
