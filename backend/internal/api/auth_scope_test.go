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
		{"DELETE", "/api/v1/tasks/abc123"},          // no destructive delete
		{"POST", "/api/v1/tasks/abc123/snooze"},     // not the create path
		{"PUT", "/api/v1/plans/2026-06-19"},         // plan is read-only for devices
		{"POST", "/api/v1/objectives"},              // objective writes
		{"GET", "/api/v1/sync/changes"},             // full changeset = too broad
		{"GET", "/api/v1/backup/settings"},          // backups
		{"POST", "/api/v1/backup/restore"},          // restore
		{"GET", "/api/v1/integrations/gmail"},       // integration config
		{"POST", "/api/v1/tags"},                    // tag writes
		{"DELETE", "/api/v1/devices/x"},             // device management
		{"GET", "/api/v1/search"},                   // search across everything
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
	full := store.create(time.Hour, "me@example.com")
	dev := store.createScoped(time.Hour, "dock", "device")

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
