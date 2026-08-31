package api

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/clevercode/sempa/internal/db"
)

func TestDeviceAllowed(t *testing.T) {
	allowed := [][2]string{
		{"GET", "/api/v1/events"},
		{"GET", "/api/v1/tasks"},
		{"POST", "/api/v1/tasks"},
		{"PATCH", "/api/v1/tasks/abc123"},
		{"GET", "/api/v1/plans/2026-06-19"},
		{"GET", "/api/v1/objectives"},
		{"GET", "/api/v1/tags"},
	}
	denied := [][2]string{
		{"DELETE", "/api/v1/tasks/abc123"},      // no destructive delete
		{"POST", "/api/v1/tasks/abc123/snooze"}, // not the create path
		{"PUT", "/api/v1/plans/2026-06-19"},     // plan is read-only for devices
		{"POST", "/api/v1/objectives"},          // objective writes
		{"GET", "/api/v1/sync/changes"},         // full changeset = too broad
		{"GET", "/api/v1/backup/settings"},      // backups
		{"POST", "/api/v1/backup/restore"},      // restore
		{"GET", "/api/v1/integrations/gmail"},   // integration config
		{"POST", "/api/v1/tags"},                // tag writes
		{"DELETE", "/api/v1/devices/x"},         // device management
		{"GET", "/api/v1/search"},               // search across everything
	}
	for _, c := range allowed {
		if !deviceAllowed(c[0], c[1]) {
			t.Errorf("expected ALLOWED: %s %s", c[0], c[1])
		}
	}
	for _, c := range denied {
		if deviceAllowed(c[0], c[1]) {
			t.Errorf("expected DENIED: %s %s", c[0], c[1])
		}
	}
}

func TestSessionScopeRoundTrip(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	store := newSessionStore(conn)
	full := store.create(time.Hour, "me@example.com", "u1")
	dev := store.createScoped(time.Hour, "dock", "", "device")

	if e, ok := store.get(full); !ok || e.Scope != "full" {
		t.Fatalf("full session scope = %q ok=%v", e.Scope, ok)
	}
	if e, ok := store.get(dev); !ok || e.Scope != "device" {
		t.Fatalf("device session scope = %q ok=%v", e.Scope, ok)
	}

	// Scope must survive a reload (the migration column persists it).
	reloaded := newSessionStore(conn)
	if e, ok := reloaded.get(dev); !ok || e.Scope != "device" {
		t.Fatalf("after reload, device scope = %q ok=%v", e.Scope, ok)
	}
}

// A session in regular use must never hard-expire out from under a client: get()
// slides its expiry forward. Regression test for the Windows client that sat at
// 401 for 11 days because its 30-day session lapsed and reap() deleted the row.
func TestSessionSlidingRenewal(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	store := newSessionStore(conn)
	full := store.create(sessionTTL, "me@example.com", "u1")
	dev := store.createScoped(deviceTokenTTL, "dock", "", "device")

	// Freshly minted: nothing renews yet, so no needless DB write per request.
	before, _ := store.get(full)
	if again, _ := store.get(full); !again.Expires.Equal(before.Expires) {
		t.Fatalf("session renewed too eagerly: %v -> %v", before.Expires, again.Expires)
	}

	// Backdate past the renew interval — the next use must slide it forward.
	store.mu.Lock()
	e := store.entries[full]
	e.Renewed = time.Now().Add(-sessionRenewInterval - time.Minute)
	store.entries[full] = e
	store.mu.Unlock()

	renewed, ok := store.get(full)
	if !ok {
		t.Fatal("session vanished")
	}
	if !renewed.Expires.After(before.Expires) {
		t.Fatalf("expected renewal, expires still %v", renewed.Expires)
	}

	// ...and the slide must be persisted, or a restart would resurrect the old
	// expiry and reap() could still delete a live session.
	var stored string
	if err := conn.QueryRow(`SELECT expires_at FROM sessions WHERE id = ?`, full).Scan(&stored); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored != renewed.Expires.UTC().Format(sessionTimeFmt) {
		t.Fatalf("expiry not persisted: db=%q want=%q", stored, renewed.Expires.UTC().Format(sessionTimeFmt))
	}

	// Device (Dock) tokens must still expire on schedule — a lost device has to
	// stop working, so they are deliberately excluded from renewal.
	store.mu.Lock()
	d := store.entries[dev]
	d.Renewed = time.Now().Add(-sessionRenewInterval - time.Minute)
	devExpires := d.Expires
	store.entries[dev] = d
	store.mu.Unlock()

	if got, _ := store.get(dev); !got.Expires.Equal(devExpires) {
		t.Fatalf("device token renewed (%v -> %v); it must expire", devExpires, got.Expires)
	}
}
