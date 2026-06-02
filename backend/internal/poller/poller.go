package poller

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/fastmail"
)

// StartInbox polls the task_inbox integration on the given interval.
// It returns immediately; polling runs in the background until ctx is cancelled.
func StartInbox(ctx context.Context, database *sql.DB, interval time.Duration) {
	if interval < time.Minute {
		interval = time.Minute
	}
	go func() {
		// Run once immediately on startup, then on each tick.
		pollInbox(ctx, database)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pollInbox(ctx, database)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func pollInbox(ctx context.Context, database *sql.DB) {
	configs := db.NewIntegrationConfigStore(database)
	tasks := db.NewTaskStore(database)

	cfg, err := configs.Get(ctx, "task_inbox")
	if err != nil {
		return // not configured — silent
	}

	var inboxCfg fastmail.InboxConfig
	if err := json.Unmarshal([]byte(cfg.Config), &inboxCfg); err != nil {
		slog.Error("inbox poller: bad config", "err", err)
		return
	}

	result, err := fastmail.SyncTaskInbox(ctx, inboxCfg, tasks)
	if err != nil {
		slog.Error("inbox poller: sync failed", "err", err)
		return
	}
	if result.New > 0 || result.Errors > 0 {
		slog.Info("inbox poller: sync complete", "new", result.New, "errors", result.Errors)
	}
	_ = configs.TouchSyncTime(ctx, "task_inbox")
}
