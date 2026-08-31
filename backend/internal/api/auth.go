package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
)

const sessionCookieName = "sempa_session"

// ── Login throttle (brute-force hardening) ────────────────────────────────────
// Per-key (IP+username) sliding counter: after maxLoginAttempts failures within
// the window, further attempts are refused until the window passes. In-memory is
// fine for a single-process self-hosted server.
const (
	maxLoginAttempts = 8
	loginWindow      = 15 * time.Minute
)

type loginThrottle struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newLoginThrottle() *loginThrottle {
	t := &loginThrottle{hits: make(map[string][]time.Time)}
	go func() {
		for range time.Tick(loginWindow) {
			now := time.Now()
			t.mu.Lock()
			for k, ts := range t.hits {
				kept := ts[:0]
				for _, x := range ts {
					if now.Sub(x) < loginWindow {
						kept = append(kept, x)
					}
				}
				if len(kept) == 0 {
					delete(t.hits, k)
				} else {
					t.hits[k] = kept
				}
			}
			t.mu.Unlock()
		}
	}()
	return t
}

// blocked reports whether the key has exceeded the attempt budget.
func (t *loginThrottle) blocked(key string) bool {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	var recent []time.Time
	for _, x := range t.hits[key] {
		if now.Sub(x) < loginWindow {
			recent = append(recent, x)
		}
	}
	t.hits[key] = recent
	return len(recent) >= maxLoginAttempts
}

func (t *loginThrottle) fail(key string) {
	t.mu.Lock()
	t.hits[key] = append(t.hits[key], time.Now())
	t.mu.Unlock()
}

func (t *loginThrottle) reset(key string) {
	t.mu.Lock()
	delete(t.hits, key)
	t.mu.Unlock()
}

// ── Session store ─────────────────────────────────────────────────────────────

type sessionEntry struct {
	Email   string
	UserID  string // users.id (empty for the device/dock scope and legacy rows)
	Expires time.Time
	// Scope gates what the session may do. "full" = a normal logged-in client
	// (everything). "device" = a paired device (the Dock): a tight allowlist of
	// today/week task + plan reads and task add/toggle, nothing sensitive — so a
	// lost/stolen device token can't take over the account. See deviceAllowed.
	Scope string
	// Renewed is when this session was last minted or slid forward. In-memory
	// only (not persisted): after a restart it's reset to load time, which just
	// delays the next renewal by up to sessionRenewInterval. Tracked explicitly
	// rather than derived from Expires so a session minted with a non-default
	// TTL is never silently promoted to a full sessionTTL.
	Renewed time.Time
}

type sessionStore struct {
	mu      sync.Mutex
	entries map[string]sessionEntry
	db      *sql.DB // optional persistence so sessions survive restarts
}

// newSessionStore builds a session store. When a DB is provided, sessions are
// persisted and reloaded on startup so a backend restart/redeploy does not log
// everyone out (the in-memory map alone is wiped on every process restart).
func newSessionStore(database *sql.DB) *sessionStore {
	s := &sessionStore{entries: make(map[string]sessionEntry), db: database}
	s.loadFromDB()
	go s.reap()
	return s
}

const sessionTimeFmt = time.RFC3339

// sessionTTL is the lifetime of a normal login session, and also the amount a
// session is extended by each time it renews (see get).
const sessionTTL = 30 * 24 * time.Hour

// sessionRenewInterval throttles sliding renewal: an actively-used session is
// extended back to a full sessionTTL at most once per interval. Without this,
// every authenticated request would issue a DB write.
//
// Why sliding renewal exists at all: sessions used to hard-expire at exactly
// sessionTTL with no way to refresh, and reap() then DELETEd the row. A client
// that had been syncing happily for 30 days would start getting 401s forever,
// with no signal to re-authenticate — it just retried the dead token every 30s.
// A client in regular use should never be logged out.
const sessionRenewInterval = 24 * time.Hour

func (s *sessionStore) loadFromDB() {
	if s.db == nil {
		return
	}
	rows, err := s.db.Query(
		`SELECT id, email, user_id, expires_at, scope FROM sessions WHERE expires_at > ?`,
		time.Now().UTC().Format(sessionTimeFmt))
	if err != nil {
		return
	}
	defer rows.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	for rows.Next() {
		var id, email, userID, exp, scope string
		if err := rows.Scan(&id, &email, &userID, &exp, &scope); err != nil {
			continue
		}
		t, err := time.Parse(sessionTimeFmt, exp)
		if err != nil {
			continue
		}
		if scope == "" {
			scope = "full"
		}
		s.entries[id] = sessionEntry{Email: email, UserID: userID, Expires: t, Scope: scope, Renewed: time.Now()}
	}
}

// create mints a normal full-access session (cookie/Bearer login).
func (s *sessionStore) create(ttl time.Duration, email, userID string) string {
	return s.createScoped(ttl, email, userID, "full")
}

// createScoped mints a session with an explicit scope ("full" or "device").
func (s *sessionStore) createScoped(ttl time.Duration, email, userID, scope string) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	expires := time.Now().Add(ttl)
	s.mu.Lock()
	s.entries[id] = sessionEntry{Email: email, UserID: userID, Expires: expires, Scope: scope, Renewed: time.Now()}
	s.mu.Unlock()
	if s.db != nil {
		_, _ = s.db.Exec(
			`INSERT OR REPLACE INTO sessions (id, email, user_id, expires_at, scope) VALUES (?, ?, ?, ?, ?)`,
			id, email, userID, expires.UTC().Format(sessionTimeFmt), scope)
	}
	return id
}

// get returns the session for id, and slides its expiry forward when it's in
// active use. Only "full" sessions renew — a "device" (Dock) token has a
// deliberately short TTL so a lost device stops working, and must still expire.
func (s *sessionStore) get(id string) (sessionEntry, bool) {
	now := time.Now()
	s.mu.Lock()
	e, ok := s.entries[id]
	if !ok || now.After(e.Expires) {
		s.mu.Unlock()
		return sessionEntry{}, false
	}
	var renewed time.Time
	if e.Scope == "full" && now.Sub(e.Renewed) >= sessionRenewInterval {
		renewed = now.Add(sessionTTL)
		e.Expires = renewed
		e.Renewed = now
		s.entries[id] = e
	}
	s.mu.Unlock()

	// Persist outside the lock — this runs at most once per sessionRenewInterval
	// per session, so it stays off the hot path for normal requests.
	if !renewed.IsZero() && s.db != nil {
		_, _ = s.db.Exec(`UPDATE sessions SET expires_at = ? WHERE id = ?`,
			renewed.UTC().Format(sessionTimeFmt), id)
	}
	return e, true
}

func (s *sessionStore) delete(id string) {
	s.mu.Lock()
	delete(s.entries, id)
	s.mu.Unlock()
	if s.db != nil {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	}
}

func (s *sessionStore) reap() {
	for range time.Tick(10 * time.Minute) {
		now := time.Now()
		s.mu.Lock()
		for id, e := range s.entries {
			if now.After(e.Expires) {
				delete(s.entries, id)
			}
		}
		s.mu.Unlock()
		if s.db != nil {
			_, _ = s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`,
				now.UTC().Format(sessionTimeFmt))
		}
	}
}

// ── OAuth state store (anti-CSRF) ─────────────────────────────────────────────

type stateEntry struct {
	Redirect        string
	AppReturnPrefix string // e.g. "com.clevercode.sempa://login" or "https://tauri.localhost/login"
	Expires         time.Time
}

type stateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
}

func newStateStore() *stateStore {
	s := &stateStore{entries: make(map[string]stateEntry)}
	go s.reap()
	return s
}

func (s *stateStore) create(redirect string, appReturnPrefix string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	s.mu.Lock()
	s.entries[id] = stateEntry{Redirect: redirect, AppReturnPrefix: appReturnPrefix, Expires: time.Now().Add(15 * time.Minute)}
	s.mu.Unlock()
	return id
}

func (s *stateStore) pop(id string) (stateEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok || time.Now().After(e.Expires) {
		return stateEntry{}, false
	}
	delete(s.entries, id)
	return e, true
}

func (s *stateStore) reap() {
	for range time.Tick(5 * time.Minute) {
		now := time.Now()
		s.mu.Lock()
		for id, e := range s.entries {
			if now.After(e.Expires) {
				delete(s.entries, id)
			}
		}
		s.mu.Unlock()
	}
}

// ── Link token store (one-time native OAuth exchange) ─────────────────────────

type linkTokenStore struct {
	mu      sync.Mutex
	entries map[string]struct {
		SessionID string
		Expires   time.Time
	}
}

func newLinkTokenStore() *linkTokenStore {
	s := &linkTokenStore{entries: make(map[string]struct {
		SessionID string
		Expires   time.Time
	})}
	go func() {
		for range time.Tick(5 * time.Minute) {
			now := time.Now()
			s.mu.Lock()
			for k, e := range s.entries {
				if now.After(e.Expires) {
					delete(s.entries, k)
				}
			}
			s.mu.Unlock()
		}
	}()
	return s
}

func (s *linkTokenStore) create(sessionID string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	s.mu.Lock()
	s.entries[tok] = struct {
		SessionID string
		Expires   time.Time
	}{SessionID: sessionID, Expires: time.Now().Add(2 * time.Minute)}
	s.mu.Unlock()
	return tok
}

func (s *linkTokenStore) pop(tok string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[tok]
	if !ok || time.Now().After(e.Expires) {
		return "", false
	}
	delete(s.entries, tok)
	return e.SessionID, true
}

// ── Auth handler ──────────────────────────────────────────────────────────────

type authHandler struct {
	cfg        config.Config
	sessions   *sessionStore
	states     *stateStore
	linkTokens *linkTokenStore
	users      *db.UserStore
	throttle   *loginThrottle
}

func newAuthHandler(cfg config.Config, database *sql.DB) *authHandler {
	return &authHandler{
		cfg:        cfg,
		sessions:   newSessionStore(database),
		states:     newStateStore(),
		linkTokens: newLinkTokenStore(),
		users:      db.NewUserStore(database),
		throttle:   newLoginThrottle(),
	}
	// No startup seed on purpose: the FIRST login after this deploys becomes the
	// admin (env-credential login, or the first Google sign-in when no users
	// exist). Seeding here would pre-create a user and rob a Google-only owner of
	// admin. resolveUser's email fallback keeps existing sessions working.
}

// passwordEnabled is true when password login is usable: either the env bootstrap
// credential is set, OR at least one user has a password (so admin-created
// accounts can sign in even without the env credential).
func (h *authHandler) passwordEnabled() bool {
	if h.cfg.AuthPassword != "" {
		return true
	}
	n, err := h.users.CountWithPassword(context.Background())
	return err == nil && n > 0
}

func (h *authHandler) envPasswordSet() bool { return h.cfg.AuthPassword != "" }
func (h *authHandler) googleEnabled() bool {
	return h.cfg.GmailClientID != "" && h.cfg.GmailClientSecret != ""
}

// authEnabled returns true when any auth mechanism is configured.
func (h *authHandler) authEnabled() bool { return h.passwordEnabled() || h.googleEnabled() }

// emailOnAllowList reports EXPLICIT membership of the env allow-list. Note the
// closed-by-default semantics: an empty list means "nobody is allowed via the
// list" (the Google gate then relies on UI invites + first-user bootstrap). This
// is deliberately different from a fall-open "empty = everyone" — that would let
// any Google account create itself on the server.
func (h *authHandler) emailOnAllowList(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, a := range h.cfg.AllowedEmails {
		if a == email {
			return true
		}
	}
	return false
}

func (h *authHandler) googleCallbackURL() string {
	return h.cfg.AppURL + "/api/v1/auth/google/callback"
}

// extractSession checks for auth in this order:
// 1. sempa_session cookie (web)
// 2. Authorization: Bearer <token> header (Tauri desktop)
// 3. ?token=<value> query param (SSE EventSource — can't set headers)
func (h *authHandler) extractSession(r *http.Request) (sessionEntry, bool) {
	// Cookie
	if c, err := r.Cookie(sessionCookieName); err == nil {
		if e, ok := h.sessions.get(c.Value); ok {
			return e, true
		}
	}
	// Authorization: Bearer
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if e, ok := h.sessions.get(token); ok {
			return e, true
		}
	}
	// Query param (for SSE EventSource)
	if token := r.URL.Query().Get("token"); token != "" {
		if e, ok := h.sessions.get(token); ok {
			return e, true
		}
	}
	return sessionEntry{}, false
}

func (h *authHandler) setSessionCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
	})
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	if !h.authEnabled() {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if !h.passwordEnabled() {
		respondError(w, http.StatusBadRequest, "password auth is not configured; use Google Sign-In")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Brute-force throttle, keyed by client IP + the attempted username.
	key := clientIP(r) + "|" + normalizeEmailLower(req.Username)
	if h.throttle.blocked(key) {
		respondError(w, http.StatusTooManyRequests, "too many attempts — try again later")
		return
	}

	email, userID, ok := h.verifyPassword(r.Context(), req.Username, req.Password)
	if !ok {
		h.throttle.fail(key)
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	h.throttle.reset(key)

	id := h.sessions.create(sessionTTL, email, userID)
	h.setSessionCookie(w, id)
	respond(w, http.StatusOK, map[string]any{"status": "ok", "token": id})
}

// verifyPassword checks credentials against (1) the env bootstrap admin (always
// available, never locks you out) and (2) password users in the DB (bcrypt).
// Returns the canonical email + user id on success.
func (h *authHandler) verifyPassword(ctx context.Context, username, password string) (email, userID string, ok bool) {
	// 1. Env bootstrap admin — unchanged, constant-time. On success, ensure a
	//    matching admin user row exists so it carries a stable id for ownership.
	if h.envPasswordSet() {
		userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(h.cfg.AuthUsername)) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(h.cfg.AuthPassword)) == 1
		if userMatch && passMatch {
			u, err := h.users.EnsureByEmail(ctx, normalizeEmailLower(h.cfg.AuthUsername), h.cfg.AuthUsername, true)
			if err != nil {
				// DB hiccup shouldn't lock out the bootstrap admin.
				return normalizeEmailLower(h.cfg.AuthUsername), "", true
			}
			return u.Email, u.ID, true
		}
	}
	// 2. Password user in the DB.
	u, err := h.users.GetByEmail(ctx, username)
	if err != nil || u.PasswordHash == nil || *u.PasswordHash == "" {
		// Constant-work compare so timing doesn't reveal whether the user exists.
		db.CheckPasswordDummy(password)
		return "", "", false
	}
	if db.CheckPassword(*u.PasswordHash, password) {
		return u.Email, u.ID, true
	}
	return "", "", false
}

func normalizeEmailLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		h.sessions.delete(c.Value)
	}
	// Mirror the attributes of the cookie set in setSessionCookie so the
	// deletion is treated identically by the browser (and so static analysis
	// sees the sensitive cookie consistently flagged HttpOnly/Secure).
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	if !h.authEnabled() {
		respond(w, http.StatusOK, map[string]any{
			"authenticated":  true,
			"auth_enabled":   false,
			"google_enabled": h.googleEnabled(),
		})
		return
	}
	entry, ok := h.extractSession(r)
	if !ok {
		respond(w, http.StatusOK, map[string]any{
			"authenticated":  false,
			"auth_enabled":   true,
			"google_enabled": h.googleEnabled(),
		})
		return
	}
	resp := map[string]any{
		"authenticated":  true,
		"auth_enabled":   true,
		"google_enabled": h.googleEnabled(),
		"email":          entry.Email,
		"user_id":        entry.UserID,
	}
	if u, ok := h.resolveUser(r.Context(), entry); ok {
		resp["user_id"] = u.ID
		resp["name"] = u.Name
		resp["is_admin"] = u.IsAdmin
	}
	// multi_user drives whether clients surface the Private/Shared controls — no
	// point cluttering a solo install with sharing UI.
	if n, err := h.users.Count(r.Context()); err == nil {
		resp["multi_user"] = n > 1
	}
	respond(w, http.StatusOK, resp)
}

// resolveUser maps a session to its user row, by id when present, else by email
// — so sessions minted before the multi-user migration (no user_id) still
// resolve without forcing a re-login.
func (h *authHandler) resolveUser(ctx context.Context, e sessionEntry) (db.User, bool) {
	if e.UserID != "" {
		if u, err := h.users.GetByID(ctx, e.UserID); err == nil {
			return u, true
		}
	}
	if e.Email != "" {
		if u, err := h.users.GetByEmail(ctx, e.Email); err == nil {
			return u, true
		}
	}
	return db.User{}, false
}

// ── Current-user context ──────────────────────────────────────────────────────

type ctxKey int

const (
	userCtxKey  ctxKey = 0
	ownerCtxKey ctxKey = 1
)

// currentSession returns the session entry attached by requireAuth, if any.
func currentSession(r *http.Request) (sessionEntry, bool) {
	e, ok := r.Context().Value(userCtxKey).(sessionEntry)
	return e, ok
}

// ownerID returns the data-ownership scope for a request, resolved once by
// requireAuth and the single source every data store uses to filter rows:
//   - a real user id   → that user sees their own rows + anything shared
//   - db.SystemScope   → unscoped (auth disabled = single-user, sees everything)
//   - "" (unset)       → fails CLOSED (matches nothing), so a missing scope can
//     never leak the whole household's data
func ownerID(r *http.Request) string {
	if v, ok := r.Context().Value(ownerCtxKey).(string); ok {
		return v
	}
	return ""
}

// resolveOwnerScope decides a request's ownership scope. Auth disabled → the
// instance is single-user, so everything is visible (SystemScope). Otherwise the
// session's user; a device/Dock session without a user id maps to the primary
// (household head) account so the Dock shows their data.
func (h *authHandler) resolveOwnerScope(r *http.Request, entry sessionEntry) string {
	if !h.authEnabled() {
		return db.SystemScope
	}
	if u, ok := h.resolveUser(r.Context(), entry); ok {
		return u.ID
	}
	if entry.Scope == "device" {
		if id, err := h.users.PrimaryID(r.Context()); err == nil && id != "" {
			return id
		}
	}
	return "" // fail closed
}

func (h *authHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.authEnabled() {
			// Single-user, no auth: everything is visible (unscoped).
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ownerCtxKey, db.SystemScope)))
			return
		}
		entry, ok := h.extractSession(r)
		if !ok {
			respondError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		// A device-scoped (paired Dock) token may only touch the tight set the
		// Dock needs; everything else is forbidden so a leaked device token can't
		// reach integrations, backups, settings, or destructive actions.
		if entry.Scope == "device" && !deviceAllowed(r.Method, r.URL.Path) {
			respondError(w, http.StatusForbidden, "not permitted for this device")
			return
		}
		// Expose the session (incl. UserID) and the resolved data-ownership scope
		// to handlers — the basis for per-user data scoping.
		ctx := context.WithValue(r.Context(), userCtxKey, entry)
		ctx = context.WithValue(ctx, ownerCtxKey, h.resolveOwnerScope(r, entry))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// deviceAllowed is the allowlist for "device"-scoped sessions (the Sempa Dock):
// read today/this-week tasks + plan + objectives, the realtime stream, and
// add/complete tasks. Anything not listed is denied. Keep in lockstep with what
// the /dock UI actually calls.
func deviceAllowed(method, path string) bool {
	switch {
	// Realtime stream.
	case method == "GET" && path == "/api/v1/events":
		return true
	// Task reads + add (create) + complete/reorder (PATCH /tasks/{id}).
	case method == "GET" && path == "/api/v1/tasks":
		return true
	case method == "POST" && path == "/api/v1/tasks":
		return true
	case method == "PATCH" && strings.HasPrefix(path, "/api/v1/tasks/"):
		return true
	// Daily plan (intention) + weekly objective + tags — read only.
	case method == "GET" && strings.HasPrefix(path, "/api/v1/plans/"):
		return true
	case method == "GET" && path == "/api/v1/objectives":
		return true
	case method == "GET" && path == "/api/v1/tags":
		return true
	default:
		return false
	}
}

// ── Google OAuth ──────────────────────────────────────────────────────────────

func (h *authHandler) googleAuth(w http.ResponseWriter, r *http.Request) {
	if !h.googleEnabled() {
		http.Error(w, "Google sign-in is not configured", http.StatusServiceUnavailable)
		return
	}
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" || !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
		redirect = "/"
	}

	// Determine where to redirect after OAuth. Each native client passes its expected
	// return scheme so the callback can redirect back into the correct app origin.
	// Origins are validated against known-safe values to prevent open redirect.
	appReturnPrefix := ""
	qs := r.URL.Query()
	switch {
	case qs.Get("native") == "true":
		// Android Chrome Custom Tab: return via custom URL scheme deep link
		appReturnPrefix = "com.clevercode.sempa://login"
	case qs.Get("desktop_deeplink") == "true":
		// Linux/macOS desktop: OAuth runs in the SYSTEM browser (WebKitGTK refuses
		// to redirect a webview to a non-HTTPS scheme like tauri://), so return via
		// the sempa:// deep link, which the already-running app picks up.
		appReturnPrefix = "sempa://login"
	case qs.Get("tauri") == "true":
		// Tauri desktop WebView: return to the Tauri localhost origin
		raw := qs.Get("tauri_origin")
		switch raw {
		case "https://tauri.localhost", "tauri://localhost", "http://tauri.localhost":
			appReturnPrefix = raw + "/login"
		default:
			appReturnPrefix = "https://tauri.localhost/login"
		}
	case qs.Get("capacitor_origin") != "":
		// Android Capacitor WebView navigation (fallback when Browser plugin unavailable)
		raw := qs.Get("capacitor_origin")
		switch raw {
		case "https://localhost", "http://localhost", "capacitor://localhost":
			appReturnPrefix = raw + "/login"
		default:
			appReturnPrefix = "https://localhost/login"
		}
	}

	state := h.states.create(redirect, appReturnPrefix)

	q := url.Values{
		"client_id":     {h.cfg.GmailClientID},
		"redirect_uri":  {h.googleCallbackURL()},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"access_type":   {"online"},
		"prompt":        {"select_account"},
	}
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+q.Encode(), http.StatusFound)
}

func (h *authHandler) googleCallback(w http.ResponseWriter, r *http.Request) {
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, "/login?error="+url.QueryEscape(errParam), http.StatusFound)
		return
	}

	state := r.URL.Query().Get("state")
	stateVal, ok := h.states.pop(state)
	if !ok {
		http.Error(w, "invalid or expired state — please try signing in again", http.StatusBadRequest)
		return
	}
	redirect := stateVal.Redirect

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code from Google", http.StatusBadRequest)
		return
	}

	accessToken, err := h.exchangeCode(r, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusBadGateway)
		return
	}

	email, err := getGoogleEmail(r.Context(), accessToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("userinfo failed: %v", err), http.StatusBadGateway)
		return
	}

	// Closed by default. A Google account may sign in only if it was INVITED (an
	// admin pre-created the row in the UI), it's on the env allow-list, or it's the
	// very first account on a fresh server (bootstrap). An empty allow-list no
	// longer means "anyone with a Google account" — invites are the front door.
	existing, _ := h.users.GetByEmail(r.Context(), email)
	invited := existing.ID != ""
	userCount, _ := h.users.Count(r.Context())
	firstUser := !invited && userCount == 0
	if !invited && !firstUser && !h.emailOnAllowList(email) {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("not_allowed"), http.StatusFound)
		return
	}

	// First user on a fresh server becomes admin; invited members are regular
	// unless an admin marked them admin when inviting.
	user, err := h.users.EnsureByEmail(r.Context(), email, "", firstUser)
	if err != nil {
		http.Error(w, "failed to provision user", http.StatusInternalServerError)
		return
	}

	id := h.sessions.create(sessionTTL, user.Email, user.ID)

	if stateVal.AppReturnPrefix != "" {
		// Native client (Android custom scheme, Tauri WebView, or Capacitor WebView):
		// issue a short-lived link token and redirect back into the app.
		// The app's login page exchanges the token for a session via /auth/native/finalize.
		lt := h.linkTokens.create(id)
		retq := url.Values{"link_token": {lt}, "redirect": {redirect}}
		http.Redirect(w, r, stateVal.AppReturnPrefix+"?"+retq.Encode(), http.StatusFound)
		return
	}

	h.setSessionCookie(w, id)
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (h *authHandler) nativeFinalize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LinkToken string `json:"link_token"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	sessionID, ok := h.linkTokens.pop(req.LinkToken)
	if !ok {
		respondError(w, http.StatusUnauthorized, "invalid or expired link token")
		return
	}
	// SameSite=None so the Capacitor WebView (origin http://localhost) can send
	// this cookie on cross-origin requests to the API server.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
	})
	// Also return the session ID in the body so Tauri desktop can store it as a
	// Bearer token (cross-origin cookies don't work in the Tauri WebView).
	respond(w, http.StatusOK, map[string]any{"status": "ok", "token": sessionID})
}

func (h *authHandler) exchangeCode(r *http.Request, code string) (string, error) {
	body := url.Values{
		"code":          {code},
		"client_id":     {h.cfg.GmailClientID},
		"client_secret": {h.cfg.GmailClientSecret},
		"redirect_uri":  {h.googleCallbackURL()},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		"https://oauth2.googleapis.com/token", bytes.NewBufferString(body.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, raw)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return "", err
	}
	if tok.Error != "" {
		return "", fmt.Errorf("google: %s", tok.Error)
	}
	return tok.AccessToken, nil
}

func getGoogleEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("userinfo HTTP %d: %s", resp.StatusCode, body)
	}

	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.Email == "" {
		return "", fmt.Errorf("no email in Google userinfo response")
	}
	return info.Email, nil
}
