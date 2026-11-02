package api

// Regression tests for the 2026-11 security audit. Each test drives the real
// router with an admin and a non-admin member and pins one boundary that a
// finding showed was missing.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/clevercode/sempa/internal/blob"
	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
)

type secFixture struct {
	t        *testing.T
	conn     *sql.DB
	srv      *httptest.Server
	adminID  string
	memberID string
}

func newSecFixture(t *testing.T) *secFixture {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "sec.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	users := db.NewUserStore(conn)
	adminHash, _ := db.HashPassword("admin-pass-1")
	memberHash, _ := db.HashPassword("member-pass-1")
	a, err := users.Create(ctx, db.CreateUserParams{Email: "a@example.test", Name: "Admin", PasswordHash: &adminHash, IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	// Ensure a deterministic primary (oldest) user.
	_, _ = conn.Exec(`UPDATE users SET created_at = '2020-01-01 00:00:00' WHERE id = ?`, a.ID)
	m, err := users.Create(ctx, db.CreateUserParams{Email: "b@example.test", Name: "Member", PasswordHash: &memberHash})
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewRouter(conn, config.Config{Env: "development", AuthUsername: "root", AuthPassword: "root-pass-1"}, blobs, ""))
	t.Cleanup(srv.Close)
	return &secFixture{t: t, conn: conn, srv: srv, adminID: a.ID, memberID: m.ID}
}

func (f *secFixture) login(email, pass string) string {
	f.t.Helper()
	status, out := f.do("", http.MethodPost, "/api/v1/auth/login", map[string]string{"username": email, "password": pass})
	tok, _ := out["token"].(string)
	if status != http.StatusOK || tok == "" {
		f.t.Fatalf("login %s: %d %v", email, status, out)
	}
	return tok
}

func (f *secFixture) do(token, method, path string, body any) (int, map[string]any) {
	f.t.Helper()
	code, raw := f.raw(token, method, path, body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return code, out
}

func (f *secFixture) raw(token, method, path string, body any) (int, []byte) {
	f.t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, f.srv.URL+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// Instance integrations and settings are admin-only; members keep read-only
// status probes, and secrets never leave the server.
func TestSecMemberCannotTouchInstanceConfig(t *testing.T) {
	f := newSecFixture(t)
	ctx := context.Background()
	cfgs := db.NewIntegrationConfigStore(f.conn)
	_, _ = cfgs.Upsert(ctx, "n1", "notifications",
		`{"master_enabled":true,"webhook_enabled":true,"webhook":{"endpoint":"https://ntfy.example.test/x","auth_header":"Authorization","auth_value":"Bearer SECRET-1"}}`)
	member := f.login("b@example.test", "member-pass-1")

	for _, c := range []struct{ method, path string }{
		{http.MethodPut, "/api/v1/integrations/caldav"},
		{http.MethodPut, "/api/v1/integrations/jira"},
		{http.MethodDelete, "/api/v1/integrations/fastmail"},
		{http.MethodGet, "/api/v1/integrations/fastmail/emails"},
		{http.MethodPost, "/api/v1/integrations/ai-title/pull"},
		{http.MethodGet, "/api/v1/integrations/gmail/auth"},
		{http.MethodPut, "/api/v1/notifications/settings"},
		{http.MethodPost, "/api/v1/notifications/webhook/test"},
		{http.MethodGet, "/api/v1/devices"},
	} {
		if code, _ := f.do(member, c.method, c.path, map[string]any{}); code != http.StatusForbidden {
			t.Errorf("member %s %s = %d, want 403", c.method, c.path, code)
		}
	}
	for _, p := range []string{"/api/v1/integrations/jira", "/api/v1/integrations/ai-title"} {
		if code, _ := f.do(member, http.MethodGet, p, nil); code != http.StatusOK {
			t.Errorf("member GET %s = %d, want 200 (status probe)", p, code)
		}
	}
	code, raw := f.raw(member, http.MethodGet, "/api/v1/notifications/settings", nil)
	if code != http.StatusOK || strings.Contains(string(raw), "SECRET-1") {
		t.Fatalf("settings read: %d, leaks secret=%v", code, strings.Contains(string(raw), "SECRET-1"))
	}

	// The admin's form round-trips the redacted blank; the secret is kept.
	admin := f.login("a@example.test", "admin-pass-1")
	var st map[string]any
	_, st = f.do(admin, http.MethodGet, "/api/v1/notifications/settings", nil)
	if code, _ := f.do(admin, http.MethodPut, "/api/v1/notifications/settings", st); code != http.StatusOK {
		t.Fatalf("admin put settings: %d", code)
	}
	row, _ := cfgs.Get(ctx, "notifications")
	if !strings.Contains(row.Config, "SECRET-1") {
		t.Fatal("redacted round-trip wiped the stored webhook secret")
	}
	// ...but moving the endpoint does not carry the secret to the new host.
	wh := st["webhook"].(map[string]any)
	wh["endpoint"] = "https://elsewhere.example.test/x"
	_, _ = f.do(admin, http.MethodPut, "/api/v1/notifications/settings", st)
	row, _ = cfgs.Get(ctx, "notifications")
	if strings.Contains(row.Config, "SECRET-1") {
		t.Fatal("secret followed the endpoint to a new host")
	}
}

// Deleting a user, changing a password and Bearer logout all end sessions.
func TestSecSessionRevocation(t *testing.T) {
	f := newSecFixture(t)
	admin := f.login("a@example.test", "admin-pass-1")

	b1 := f.login("b@example.test", "member-pass-1")
	b2 := f.login("b@example.test", "member-pass-1")
	if code, _ := f.do(b1, http.MethodPost, "/api/v1/users/password",
		map[string]string{"current_password": "member-pass-1", "new_password": "member-pass-2"}); code != http.StatusOK {
		t.Fatalf("change password: %d", code)
	}
	if code, _ := f.do(b2, http.MethodGet, "/api/v1/tasks", nil); code != http.StatusUnauthorized {
		t.Errorf("other session after password change = %d, want 401", code)
	}
	if code, _ := f.do(b1, http.MethodGet, "/api/v1/tasks", nil); code != http.StatusOK {
		t.Errorf("session that changed the password = %d, want 200", code)
	}

	if code, _ := f.raw(b1, http.MethodPost, "/api/v1/auth/logout", nil); code != http.StatusNoContent {
		t.Fatalf("logout: %d", code)
	}
	if code, _ := f.do(b1, http.MethodGet, "/api/v1/tasks", nil); code != http.StatusUnauthorized {
		t.Errorf("Bearer token after logout = %d, want 401", code)
	}

	b3 := f.login("b@example.test", "member-pass-2")
	if code, _ := f.do(admin, http.MethodDelete, "/api/v1/users/"+f.memberID, nil); code != http.StatusOK {
		t.Fatalf("delete user: %d", code)
	}
	if code, _ := f.do(b3, http.MethodPut, "/api/v1/plans/2026-11-02", map[string]string{"intention": "X"}); code != http.StatusUnauthorized {
		t.Errorf("deleted user's token = %d, want 401", code)
	}
}

// A member's ?today= rollover touches only their own recurring instances.
func TestSecRolloverIsOwnerScoped(t *testing.T) {
	f := newSecFixture(t)
	ctx := context.Background()
	ts := db.NewTaskStore(f.conn)
	rule := "daily"
	tmpl, err := ts.Create(ctx, db.CreateTaskParams{ID: "tmpl-a", Title: "A daily", Status: "backlog", RecurrenceRule: &rule, OwnerID: f.adminID, Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	today := db.ServerToday()
	if err := ts.GenerateForDate(ctx, today, f.adminID); err != nil {
		t.Fatal(err)
	}
	before, _ := ts.ListByRecurrenceOrigin(ctx, tmpl.ID, f.adminID)
	if len(before) == 0 {
		t.Fatal("no instance generated")
	}

	member := f.login("b@example.test", "member-pass-1")
	if code, _ := f.do(member, http.MethodGet, "/api/v1/tasks?date="+today+"&today=9999-12-31", nil); code != http.StatusOK {
		t.Fatalf("member list: %d", code)
	}
	after, _ := ts.ListByRecurrenceOrigin(ctx, tmpl.ID, f.adminID)
	if len(after) != len(before) {
		t.Fatalf("admin instances changed: %d -> %d", len(before), len(after))
	}
	for _, i := range after {
		if i.PlannedDate == nil || *i.PlannedDate != today {
			t.Fatalf("admin instance moved to %v", i.PlannedDate)
		}
	}
}

func TestSecClientTodayIsBounded(t *testing.T) {
	server, _ := time.Parse("2006-01-02", db.ServerToday())
	if got := clientToday(server.AddDate(0, 0, 1).Format("2006-01-02")); got == "" {
		t.Error("a neighbouring-timezone date should be accepted")
	}
	for _, s := range []string{"9999-12-31", "0001-01-01", "nope", ""} {
		if got := clientToday(s); got != "" {
			t.Errorf("clientToday(%q) = %q, want fallback", s, got)
		}
	}
}

// Reordering list items can't reach another member's private list.
func TestSecListReorderScoped(t *testing.T) {
	f := newSecFixture(t)
	admin := f.login("a@example.test", "admin-pass-1")
	member := f.login("b@example.test", "member-pass-1")
	_, l := f.do(admin, http.MethodPost, "/api/v1/lists", map[string]any{"title": "private"})
	listID, _ := l["id"].(string)
	_, it := f.do(admin, http.MethodPost, "/api/v1/lists/"+listID+"/items", map[string]any{"text": "a1"})
	itemID, _ := it["id"].(string)
	if listID == "" || itemID == "" {
		t.Fatalf("setup: list=%v item=%v", l, it)
	}
	var pos float64
	_ = f.conn.QueryRow(`SELECT position FROM list_items WHERE id = ?`, itemID).Scan(&pos)

	if code, _ := f.do(member, http.MethodPost, "/api/v1/lists/"+listID+"/items/reorder", map[string]any{"ids": []string{"x", "y", itemID}}); code != http.StatusNotFound {
		t.Errorf("member reorder of a private list = %d, want 404", code)
	}
	_, ml := f.do(member, http.MethodPost, "/api/v1/lists", map[string]any{"title": "mine"})
	mineID, _ := ml["id"].(string)
	_, _ = f.do(member, http.MethodPost, "/api/v1/lists/"+mineID+"/items/reorder", map[string]any{"ids": []string{"x", "y", itemID}})
	var after float64
	_ = f.conn.QueryRow(`SELECT position FROM list_items WHERE id = ?`, itemID).Scan(&after)
	if after != pos {
		t.Errorf("another list's item moved via my list: %v -> %v", pos, after)
	}
}

// Making a recurring template private makes its instances private too, and an
// un-share by a member revokes from the member, never from the owner.
func TestSecUnshareCascadeAndRevocationOwner(t *testing.T) {
	f := newSecFixture(t)
	ctx := context.Background()
	admin := f.login("a@example.test", "admin-pass-1")
	member := f.login("b@example.test", "member-pass-1")
	ts := db.NewTaskStore(f.conn)
	rule := "daily"
	tmpl, _ := ts.Create(ctx, db.CreateTaskParams{ID: "tmpl-s", Title: "shared daily", Status: "backlog", RecurrenceRule: &rule, OwnerID: f.adminID, Shared: true, Tags: []string{}})
	today := db.ServerToday()
	_ = ts.GenerateForDate(ctx, today, f.adminID)
	inst, _ := ts.ListByRecurrenceOrigin(ctx, tmpl.ID, f.adminID)
	if len(inst) == 0 {
		t.Fatal("no instance")
	}
	_, _ = f.conn.Exec(`UPDATE tasks SET is_customized = 1 WHERE id = ?`, inst[0].ID)

	if code, _ := f.do(admin, http.MethodPatch, "/api/v1/tasks/"+tmpl.ID, map[string]any{"shared": false}); code != http.StatusOK {
		t.Fatalf("unshare: %d", code)
	}
	if code, _ := f.do(member, http.MethodGet, "/api/v1/tasks/"+inst[0].ID, nil); code != http.StatusNotFound {
		t.Errorf("member read of a now-private instance = %d, want 404", code)
	}

	// Member un-shares a task the admin owns.
	_, tk := f.do(admin, http.MethodPost, "/api/v1/tasks", map[string]any{"title": "shared one", "shared": true})
	tid, _ := tk["id"].(string)
	if code, _ := f.do(member, http.MethodPatch, "/api/v1/tasks/"+tid, map[string]any{"shared": false}); code != http.StatusOK {
		t.Fatalf("member unshare: %d", code)
	}
	var owner string
	_ = f.conn.QueryRow(`SELECT owner_id FROM sync_tombstones WHERE entity_id = ? AND kind = 'revoke'`, tid).Scan(&owner)
	if owner != f.adminID {
		t.Errorf("revocation keyed to %q, want the row owner %q", owner, f.adminID)
	}
}

// Integration import keys and script links can't be set by clients.
func TestSecTaskCreateSanitisesSourceFields(t *testing.T) {
	f := newSecFixture(t)
	member := f.login("b@example.test", "member-pass-1")
	_, tk := f.do(member, http.MethodPost, "/api/v1/tasks", map[string]any{
		"title": "t", "source": "jira", "source_id": "SEC-2",
		"source_url":      "javascript:alert(1)",
		"source_metadata": `{"links":["javascript:alert(1)","https://ok.example.test/"],"from":"x"}`,
	})
	if tk["source"] != nil || tk["source_id"] != nil {
		t.Errorf("reserved import key kept: source=%v id=%v", tk["source"], tk["source_id"])
	}
	if tk["source_url"] != nil {
		t.Errorf("script source_url kept: %v", tk["source_url"])
	}
	meta, _ := tk["source_metadata"].(string)
	if strings.Contains(meta, "javascript:") || !strings.Contains(meta, "https://ok.example.test/") {
		t.Errorf("metadata links not filtered: %s", meta)
	}
}

// Calendar subscriptions are personal.
func TestSecICalSubscriptionsPerUser(t *testing.T) {
	f := newSecFixture(t)
	ctx := context.Background()
	member := f.login("b@example.test", "member-pass-1")
	ical := db.NewICalStore(f.conn)
	sub, err := ical.CreateSubscription(ctx, "sub-a", "A cal", "https://cal.example.test/private-TOKEN.ics", "#000", f.adminID)
	if err != nil {
		t.Fatal(err)
	}
	code, raw := f.raw(member, http.MethodGet, "/api/v1/ical/subscriptions", nil)
	if code != http.StatusOK || strings.Contains(string(raw), "private-TOKEN") {
		t.Errorf("member list: %d leaks=%v", code, strings.Contains(string(raw), "private-TOKEN"))
	}
	if code, _ := f.raw(member, http.MethodDelete, "/api/v1/ical/subscriptions/"+sub.ID, nil); code != http.StatusNotFound {
		t.Errorf("member delete of admin subscription = %d, want 404", code)
	}
}

func TestSecSafeLocalRedirect(t *testing.T) {
	for _, s := range []string{"/\\evil.example/x", "/\t/evil.example", "//evil.example", "https://evil.example", "", "settings"} {
		if got := safeLocalRedirect(s); got != "/" {
			t.Errorf("safeLocalRedirect(%q) = %q, want /", s, got)
		}
	}
	if got := safeLocalRedirect("/settings?x=1"); got != "/settings?x=1" {
		t.Errorf("local path rewritten: %q", got)
	}
}

// Push subscriptions must point at a public https push service.
func TestSecPushEndpointValidated(t *testing.T) {
	f := newSecFixture(t)
	member := f.login("b@example.test", "member-pass-1")
	for _, ep := range []string{"http://127.0.0.1:9/x", "https://127.0.0.1/x", "http://push.example.test/x"} {
		code, _ := f.do(member, http.MethodPost, "/api/v1/notifications/webpush/subscribe",
			map[string]any{"endpoint": ep, "keys": map[string]string{"p256dh": "k", "auth": "a"}})
		if code != http.StatusBadRequest {
			t.Errorf("subscribe %s = %d, want 400", ep, code)
		}
	}
}
