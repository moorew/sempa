package poller

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/ai"
	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/fastmail"
	"github.com/clevercode/sempa/internal/integrations/jira"
)

// StartInbox polls the task_inbox integration on the given interval.
func StartInbox(ctx context.Context, database *sql.DB, interval time.Duration, aiEnv db.AIEnv) {
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	go func() {
		pollInbox(ctx, database, aiEnv)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pollInbox(ctx, database, aiEnv)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func pollInbox(ctx context.Context, database *sql.DB, aiEnv db.AIEnv) {
	configs := db.NewIntegrationConfigStore(database)
	tasks := db.NewTaskStore(database)

	cfg, err := configs.Get(ctx, "task_inbox")
	if err != nil {
		return
	}

	var inboxCfg fastmail.InboxConfig
	if err := json.Unmarshal([]byte(cfg.Config), &inboxCfg); err != nil {
		slog.Error("inbox poller: bad config", "err", err)
		return
	}
	// Effective AI config (DB override, else env): local Ollama or an Aperture
	// gateway. When AI is off the zero Config makes ImproveTitle keep the raw
	// subject.
	if resolved := configs.ResolveAI(ctx, aiEnv); resolved.Enabled {
		p, u, m := resolved.Effective()
		inboxCfg.AI = ai.Config{Provider: p, BaseURL: u, Model: m}
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

	// Keep Jira synced on the same cadence so issues stay fresh and a deleted
	// Jira-linked task re-imports on its own (it used to require a manual sync).
	pollJira(ctx, database)
}

func pollJira(ctx context.Context, database *sql.DB) {
	configs := db.NewIntegrationConfigStore(database)
	cfg, err := configs.Get(ctx, "jira")
	if err != nil {
		return // not configured
	}
	var jiraCfg jira.Config
	if err := json.Unmarshal([]byte(cfg.Config), &jiraCfg); err != nil {
		return
	}
	if jiraCfg.Host == "" || jiraCfg.APIToken == "" {
		return
	}
	tasks := db.NewTaskStore(database)
	result, err := jira.Sync(ctx, jiraCfg, tasks)
	if err != nil {
		slog.Error("jira poller: sync failed", "err", err)
		return
	}
	if result.New > 0 || result.Updated > 0 || result.Errors > 0 {
		slog.Info("jira poller: sync complete", "new", result.New, "updated", result.Updated, "errors", result.Errors)
	}
	_ = configs.TouchSyncTime(ctx, "jira")
}
