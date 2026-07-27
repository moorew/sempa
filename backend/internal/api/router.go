package api

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/clevercode/sempa/internal/backup"
	"github.com/clevercode/sempa/internal/blob"
	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
)

func NewRouter(database *sql.DB, cfg config.Config, blobs *blob.Store, vapidPublicKey string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// Trusted-proxy-aware client-IP resolution — only honours X-Forwarded-For /
	// X-Real-IP from configured (or default private/loopback) proxy peers so a
	// direct client can't spoof its IP past the throttles. (AURA-SEC-004)
	r.Use(realIP(cfg.TrustedProxies))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	allowOrigin := func(_ *http.Request, origin string) bool {
		if cfg.Env != "production" {
			return strings.HasPrefix(origin, "http://localhost") ||
				strings.HasPrefix(origin, "https://localhost") ||
				strings.HasPrefix(origin, "http://127.0.0.1:")
		}
		// Allow Capacitor mobile app origins
		if origin == "http://localhost" || origin == "https://localhost" || origin == "capacitor://localhost" {
			return true
		}
		// Allow Tauri desktop app origins
		// Tauri 2 on Windows uses https://tauri.localhost (WebView2 custom scheme)
		// Tauri 2 on macOS/Linux uses tauri://localhost
		if origin == "tauri://localhost" || origin == "http://tauri.localhost" || origin == "https://tauri.localhost" {
			return true
		}
		return origin == cfg.FrontendURL
	}
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  allowOrigin,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	tagStore := db.NewTagStore(database)
	configStore := db.NewIntegrationConfigStore(database)
	setup := &setupHandler{configs: configStore}
	fmCalStore := db.NewFastmailCalStore(database)
	auth := newAuthHandler(cfg, database)
	pairing := &pairingHandler{store: db.NewPairingStore(database), sessions: auth.sessions}

	hub := NewEventHub()

	syncStore := db.NewSyncStore(database)
	objectiveStore := db.NewObjectiveStore(database)
	attachments := &attachmentHandler{
		store:      db.NewAttachmentStore(database),
		blobs:      blobs,
		tasks:      db.NewTaskStore(database),
		objectives: objectiveStore,
		hub:        hub,
	}

	listStore := db.NewListStore(database)
	tasks := &taskHandler{
		store:   db.NewTaskStore(database),
		tags:    tagStore,
		configs: configStore,
		appURL:  cfg.AppURL,
		hub:     hub,
		attach:  attachments,
		sync:    syncStore,
		lists:   listStore,
	}
	objectives := &objectiveHandler{store: objectiveStore, hub: hub, attach: attachments, sync: syncStore}
	lists := &listHandler{store: listStore, hub: hub, sync: syncStore}

	backupSvc := backup.NewService(database, cfg.DBPath, blobs.Dir())
	backups := &backupHandler{
		svc:     backupSvc,
		store:   db.NewBackupStore(database),
		configs: configStore,
		hub:     hub,
		cfg:     cfg,
	}
	plans := &planHandler{store: db.NewDailyPlanStore(database), hub: hub}
	sessions := &sessionHandler{store: db.NewSessionStore(database)}
	tags := &tagHandler{store: tagStore, hub: hub, sync: syncStore}
	weekReviews := &weekReviewHandler{store: db.NewWeekReviewStore(database)}
	syncH := &syncHandler{store: syncStore}
	icals := &icalHandler{
		store:      db.NewICalStore(database),
		fmCalStore: fmCalStore,
		configs:    configStore,
	}
	devices := &deviceHandler{store: db.NewDeviceTokenStore(database)}
	notifications := &notificationHandler{
		configs:  configStore,
		pushSubs: db.NewPushSubStore(database),
		vapidPub: vapidPublicKey,
	}
	unfurls := &unfurlHandler{store: db.NewUnfurlStore(database)}
	search := &searchHandler{
		tasks:      db.NewTaskStore(database),
		objectives: objectiveStore,
		plans:      db.NewDailyPlanStore(database),
		reviews:    db.NewWeekReviewStore(database),
	}
	integrations := &integrationHandler{
		db:         database,
		configs:    configStore,
		tasks:      db.NewTaskStore(database),
		fmCalStore: fmCalStore,
		cfg:        cfg,
		pulls:      map[string]*pullState{},
	}

	r.Route("/api/v1", func(r chi.Router) {
		// Public: health + auth endpoints
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			respond(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		// Public auth config — lets the login page know which methods are available
		// before the user has a session. Always 200, never needs a cookie.
		r.Get("/auth/config", func(w http.ResponseWriter, r *http.Request) {
			respond(w, http.StatusOK, map[string]bool{
				"google_enabled":   auth.googleEnabled(),
				"password_enabled": auth.passwordEnabled(),
			})
		})
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", auth.login)
			r.Post("/logout", auth.logout)
			r.Get("/me", auth.me)
			r.Get("/google", auth.googleAuth)
			r.Get("/google/callback", auth.googleCallback)
			r.Post("/native/finalize", auth.nativeFinalize)
		})

		// Setup status — public read so the frontend can redirect before auth
		r.Get("/setup/status", setup.status)

		// Cloudflare email webhook — token-auth, not session-auth
		r.Post("/tasks/from-email", integrations.fromEmail)

		// Gmail OAuth callback must be accessible during the redirect flow
		r.Get("/integrations/gmail/callback", integrations.gmailCallback)

		// Backup Drive OAuth callback (drive.file scope) — same redirect-flow rules
		r.Get("/backup/drive/callback", backups.driveCallback)

		// Device pairing — start/status are public (the unpaired device has no
		// token yet); approval is authenticated below. /start creates a row, so
		// it's rate-limited per IP to bound the public surface.
		pairStartLimit := newRateLimiter(10, time.Minute)
		r.Route("/devices/pair", func(r chi.Router) {
			r.With(pairStartLimit.middleware).Post("/start", pairing.start)
			r.Get("/status", pairing.status)
		})

		// All remaining API routes require session auth (if auth is configured)
		r.Group(func(r chi.Router) {
			r.Use(auth.requireAuth)

			r.Get("/events", hub.ServeSSE)

			// Global search across tasks, objectives and journal entries.
			r.Get("/search", search.search)

			// Link preview / Open Graph unfurl for URLs in task notes.
			r.Get("/unfurl", unfurls.get)
			// Same-origin proxy for preview thumbnails (robust image loading).
			r.Get("/unfurl/image", unfurls.image)

			// Offline sync: pull all changes since a cursor (created/updated/deleted)
			r.Get("/sync/changes", syncH.changes)

			// Planned-vs-actual time profile (per-tag + global multipliers).
			r.Get("/insights/time", tasks.timeInsights)
			// Raw logged durations for the client-side activity-bucket learning profile.
			r.Get("/insights/durations", tasks.durationSamples)

			// User & credential management (admin-gated inside the handlers;
			// self password change is open to any authed user).
			r.Route("/users", func(r chi.Router) {
				r.Get("/", auth.listUsers)
				r.Post("/", auth.createUser)
				r.Post("/password", auth.changeOwnPassword)
				r.Delete("/{id}", auth.deleteUser)
				r.Post("/{id}/password", auth.adminSetPassword)
			})

			// Lists — standalone checklists, optionally linked to a task.
			r.Route("/lists", func(r chi.Router) {
				r.Get("/", lists.list)
				r.Post("/", lists.create)
				r.Get("/{id}", lists.get)
				r.Patch("/{id}", lists.update)
				r.Delete("/{id}", lists.delete)
				r.Get("/{id}/items", lists.items)
				r.Post("/{id}/items", lists.createItem)
				r.Post("/{id}/items/reorder", lists.reorderItems)
			})
			r.Route("/list-items", func(r chi.Router) {
				r.Patch("/{id}", lists.updateItem)
				r.Delete("/{id}", lists.deleteItem)
			})

			r.Route("/tasks", func(r chi.Router) {
				r.Get("/", tasks.list)
				r.Post("/", tasks.create)
				r.Get("/recurring", tasks.listTemplates)
				r.Get("/recurring/instances", tasks.recurringInstances)
				r.Get("/{id}", tasks.get)
				r.Patch("/{id}", tasks.update)
				r.Delete("/{id}", tasks.delete)
				r.Post("/{id}/snooze", tasks.snooze)
				r.Get("/{id}/attachments", attachments.list("task"))
				r.Post("/{id}/attachments", attachments.upload("task"))
			})

			r.Route("/attachments", func(r chi.Router) {
				r.Get("/{id}/download", attachments.download)
				r.Delete("/{id}", attachments.delete)
			})

			// Backup is a whole-instance operation: a download bundles every
			// user's data (and, by mode, integration secrets) and restore replaces
			// all global data. Admin-only, never reachable by a regular member or a
			// paired device. (AURA-SEC-001)
			r.Route("/backup", func(r chi.Router) {
				r.Use(auth.requireAdmin)
				r.Get("/settings", backups.getSettings)
				r.Put("/settings", backups.updateSettings)
				// Cheap DB-only health read the in-app banner polls (see backups.health).
				r.Get("/health", backups.health)
				r.Get("/runs", backups.listRuns)
				r.Get("/download", backups.download)
				r.Post("/restore", backups.restore)
				r.Post("/run", backups.runNow)
				r.Post("/test", backups.testDestination)
				// Google Drive OAuth for backups (drive.file scope).
				r.Get("/drive/auth", backups.driveAuth)
				r.Get("/drive", backups.driveStatus)
				r.Delete("/drive", backups.driveDisconnect)
			})

			r.Route("/tags", func(r chi.Router) {
				r.Get("/", tags.list)
				r.Post("/", tags.create)
				r.Patch("/{id}", tags.update)
				r.Delete("/{id}", tags.delete)
			})

			r.Route("/objectives", func(r chi.Router) {
				r.Get("/", objectives.list)
				r.Post("/", objectives.create)
				r.Get("/{id}", objectives.get)
				r.Patch("/{id}", objectives.update)
				r.Delete("/{id}", objectives.delete)
				r.Get("/{id}/attachments", attachments.list("objective"))
				r.Post("/{id}/attachments", attachments.upload("objective"))
			})

			r.Route("/plans", func(r chi.Router) {
				r.Get("/", plans.list)
				r.Get("/{date}", plans.get)
				r.Put("/{date}", plans.upsert)
			})

			r.Route("/weeks", func(r chi.Router) {
				r.Get("/reviews", weekReviews.list)
				r.Get("/{weekStart}/review", weekReviews.get)
				r.Put("/{weekStart}/review", weekReviews.upsert)
			})

			r.Route("/ical", func(r chi.Router) {
				r.Get("/subscriptions", icals.listSubscriptions)
				r.Post("/subscriptions", icals.createSubscription)
				r.Delete("/subscriptions/{id}", icals.deleteSubscription)
				r.Post("/subscriptions/{id}/sync", icals.syncSubscription)
				r.Get("/events", icals.listEventsForDate)
			})

			r.Post("/setup/complete", setup.complete)

			r.Route("/pomodoros", func(r chi.Router) {
				r.Get("/", sessions.listByTask)
				r.Post("/", sessions.create)
			})

			r.Route("/devices", func(r chi.Router) {
				r.Post("/", devices.register)
				r.Delete("/", devices.unregister)
				// Paired devices (Dock): approve a code, list, revoke.
				r.Post("/pair/approve", pairing.approve)
				r.Get("/", pairing.list)
				r.Delete("/{id}", pairing.revoke)
			})

			r.Route("/notifications", func(r chi.Router) {
				r.Get("/settings", notifications.getSettings)
				r.Put("/settings", notifications.putSettings)
				r.Get("/vapid-public-key", notifications.vapidKey)
				r.Post("/webpush/subscribe", notifications.subscribe)
				r.Delete("/webpush/subscribe", notifications.unsubscribe)
				r.Post("/webhook/test", notifications.testWebhook)
			})

			r.Route("/integrations", func(r chi.Router) {
				r.Route("/jira", func(r chi.Router) {
					r.Get("/", integrations.jiraGet)
					r.Put("/", integrations.jiraPut)
					r.Delete("/", integrations.jiraDelete)
					r.Post("/test", integrations.jiraTest)
					r.Post("/sync", integrations.jiraSync)
					r.Get("/statuses", integrations.jiraGetStatuses)
					r.Get("/issues/{key}", integrations.jiraGetIssue)
					r.Get("/issues/{key}/transitions", integrations.jiraGetTransitions)
					r.Post("/issues/{key}/transition", integrations.jiraDoTransition)
				})
				r.Route("/gmail", func(r chi.Router) {
					r.Get("/", integrations.gmailGet)
					r.Delete("/", integrations.gmailDelete)
					r.Get("/auth", integrations.gmailAuth)
					r.Patch("/labels", integrations.gmailUpdateLabels)
					r.Post("/sync", integrations.gmailSync)
				})
				r.Route("/calendar", func(r chi.Router) {
					r.Get("/", integrations.calendarGet)
					r.Patch("/", integrations.calendarToggle)
					r.Post("/sync", integrations.calendarSync)
				})
				r.Route("/fastmail", func(r chi.Router) {
					r.Get("/", integrations.fastmailGet)
					r.Put("/", integrations.fastmailPut)
					r.Delete("/", integrations.fastmailDelete)
					r.Post("/sync", integrations.fastmailSync)
					r.Get("/emails", integrations.fastmailEmails)
					r.Get("/emails/archived", integrations.fastmailArchivedEmails)
					r.Post("/emails/{id}/to-task", integrations.fastmailEmailToTask)
					r.Post("/emails/{id}/archive", integrations.fastmailArchiveEmail)
					r.Post("/emails/{id}/unarchive", integrations.fastmailUnarchiveEmail)
					r.Route("/calendar", func(r chi.Router) {
						r.Get("/", integrations.fastmailCalendarGet)
						r.Get("/list", integrations.fastmailListCalendars)
						r.Patch("/", integrations.fastmailCalendarToggle)
						r.Post("/sync", integrations.fastmailCalendarSync)
					})
				})
				r.Route("/caldav", func(r chi.Router) {
					r.Get("/", integrations.caldavGet)
					r.Get("/calendars", integrations.caldavListCalendars)
					r.Put("/", integrations.caldavSelect)
					r.Patch("/", integrations.caldavToggle)
					r.Post("/sync", integrations.caldavSync)
				})
				r.Get("/email-forward", integrations.emailForwardGet)
				r.Route("/task-inbox", func(r chi.Router) {
					r.Get("/", integrations.taskInboxGet)
					r.Put("/", integrations.taskInboxPut)
					r.Patch("/senders", integrations.taskInboxPatchSenders)
					r.Post("/sync", integrations.taskInboxSync)
					r.Delete("/", integrations.taskInboxDelete)
				})
				r.Route("/ai-title", func(r chi.Router) {
					r.Get("/", integrations.aiTitleGet)
					r.Put("/", integrations.aiTitleUpdate)
					r.Post("/test", integrations.aiTitleTest)
					r.Post("/pull", integrations.aiTitlePull)
					r.Get("/pull", integrations.aiTitlePullStatus)
					r.Post("/remove", integrations.aiTitleRemove)
				})
			})

			// AI-assist features (all run on the local model; return
			// {available:false} when AI is off so the UI hides them).
			r.Route("/ai", func(r chi.Router) {
				r.Post("/quick-add", integrations.aiQuickAdd)
				r.Post("/summarize", integrations.aiSummarizeTask)
				r.Post("/suggest-tags", integrations.aiSuggestTags)
				r.Post("/breakdown", integrations.aiBreakdown)
				r.Post("/tidy-notes", integrations.aiTidyNotes)
				r.Post("/plan-day", integrations.aiPlanDay)
				r.Post("/predict-time", integrations.aiPredictTime)
				r.Post("/organize-list", integrations.aiOrganizeList)
				r.Post("/weekly-review", integrations.aiWeeklyReview)
				r.Post("/reflection-prompts", integrations.aiReflectionPrompts)
				r.Post("/import", integrations.aiImport)
			})
		})
	})

	// Serve static frontend if configured (SPA fallback to index.html)
	if cfg.FrontendDir != "" {
		r.Handle("/*", spaHandler(cfg.FrontendDir))
	}

	return r
}

// spaHandler serves static files and falls back to index.html for client-side routing.
// HTML files are sent with no-cache headers so browsers always fetch the latest entry point.
// Hashed JS/CSS assets get long-lived caching from the browser's default behaviour.
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		// Prevent caching of HTML files (SPA entry points change on every deploy)
		if strings.HasSuffix(r.URL.Path, ".html") || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-store")
		}
		fs.ServeHTTP(w, r)
	})
}
