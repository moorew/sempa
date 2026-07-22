package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port           string
	DBPath         string
	AttachmentsDir string // dir for attachment blobs; default <db-dir>/attachments
	Env            string
	FrontendDir    string // path to built static frontend; empty = API-only mode

	// Timezone is the server's home IANA timezone (e.g. "America/Toronto"), used
	// for every server-side day-boundary calculation (recurrence rollover, the
	// recurrence-horizon poller, the morning digest). Empty → UTC. The container
	// clock is UTC, so without this the day turns over at the wrong local moment.
	// Overridable at runtime by the notifications.timezone setting.
	Timezone string

	// OAuth / integration
	AppURL            string // e.g. https://blackbox.clevercode.ts.net
	FrontendURL       string // e.g. http://localhost:5173 (dev only)
	GmailClientID     string
	GmailClientSecret string

	// Inbound SMTP (email forwarding)
	SMTPPort           string   // e.g. "2525"; empty disables the SMTP server
	SMTPAllowedSenders []string // email addresses or @domain; empty = accept all

	// Auth — optional; if AuthPassword is empty, password auth is disabled
	AuthUsername string
	AuthPassword string
	// Google Sign-In: comma-separated emails allowed to log in.
	// If empty, any Google account is accepted (fine for self-hosted on Tailscale).
	AllowedEmails []string

	// Webhook token for Cloudflare Email Routing → POST /api/v1/tasks/from-email
	EmailForwardToken string

	// TrustedProxies is the set of CIDR ranges (or bare IPs) whose
	// X-Forwarded-For / X-Real-IP headers are trusted when resolving the client
	// IP for throttling. When empty, only loopback and private/ULA/CGNAT peers
	// are trusted (a reverse proxy on the same host/network) — a directly
	// connected public client can never spoof its IP to evade throttles.
	TrustedProxies []string

	// Background inbox polling interval (e.g. "1m"); empty disables
	InboxPollInterval string

	// Background calendar refresh interval (ICS subscriptions + Fastmail
	// calendar), e.g. "15m"; empty disables periodic calendar syncing.
	CalendarPollInterval string

	// Optional: Ollama base URL for local AI-powered task title cleanup
	OllamaBaseURL string // e.g. http://ollama:11434
	OllamaModel   string // default: qwen2.5:1.5b

	// Firebase Cloud Messaging — path to service account JSON key file
	FCMKeyPath string // e.g. ./firebase-service-account.json

	// VAPID contact subject for Web Push JWTs (RFC 8292 `sub` claim). The VAPID
	// key pair itself is auto-generated and persisted in the DB on first boot.
	VAPIDSubject string // e.g. mailto:you@example.com
}

func Load() Config {
	dbPath := env("DB_PATH", "./data/sempa.db")
	return Config{
		Port:                 env("PORT", "8080"),
		DBPath:               dbPath,
		AttachmentsDir:       env("ATTACHMENTS_DIR", filepath.Join(filepath.Dir(dbPath), "attachments")),
		Env:                  env("ENV", "development"),
		FrontendDir:          env("FRONTEND_DIR", ""),
		Timezone:             env("SEMPA_TIMEZONE", ""),
		AppURL:               env("APP_URL", "http://localhost:8080"),
		FrontendURL:          env("FRONTEND_URL", "http://localhost:5173"),
		GmailClientID:        env("GMAIL_CLIENT_ID", ""),
		GmailClientSecret:    env("GMAIL_CLIENT_SECRET", ""),
		SMTPPort:             env("SMTP_PORT", "2525"),
		SMTPAllowedSenders:   splitEmails(env("SMTP_ALLOWED_SENDERS", "")),
		AuthUsername:         env("SEMPA_USERNAME", "admin"),
		AuthPassword:         env("SEMPA_PASSWORD", ""),
		AllowedEmails:        splitEmails(env("SEMPA_ALLOWED_EMAILS", "")),
		EmailForwardToken:    env("EMAIL_FORWARD_TOKEN", ""),
		TrustedProxies:       splitCSV(env("SEMPA_TRUSTED_PROXIES", "")),
		InboxPollInterval:    env("INBOX_POLL_INTERVAL", "1m"),
		CalendarPollInterval: env("CALENDAR_POLL_INTERVAL", "15m"),
		OllamaBaseURL:        env("OLLAMA_BASE_URL", ""),
		OllamaModel:          env("OLLAMA_MODEL", "qwen2.5:1.5b"),
		FCMKeyPath:           env("FCM_KEY_PATH", ""),
		VAPIDSubject:         env("VAPID_SUBJECT", "mailto:admin@localhost"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitEmails(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, strings.ToLower(v))
		}
	}
	return out
}

// splitCSV splits a comma-separated list into trimmed, non-empty values,
// preserving case (used for CIDRs/IPs where lower-casing is unnecessary).
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
