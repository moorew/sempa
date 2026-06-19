package db

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestPairingStore(t *testing.T) *PairingStore {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewPairingStore(conn)
}

func TestPairingFlow(t *testing.T) {
	s := newTestPairingStore(t)

	d, err := s.CreatePending("Desk Dock", "dock", 10*time.Minute)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if d.Status != "pending" || len(d.Code) != 8 {
		t.Fatalf("unexpected pending device: %+v", d)
	}

	// Approve attaches the minted token.
	if _, err := s.Approve(d.Code, "tok-123"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// Re-approving the same (now non-pending) code must fail.
	if _, err := s.Approve(d.Code, "tok-xyz"); err != ErrNotFound {
		t.Fatalf("re-approve: want ErrNotFound, got %v", err)
	}

	// The token is handed over exactly once.
	tok, err := s.ClaimToken(d.Code)
	if err != nil || tok != "tok-123" {
		t.Fatalf("claim: tok=%q err=%v", tok, err)
	}
	if tok2, _ := s.ClaimToken(d.Code); tok2 != "" {
		t.Fatalf("second claim should be empty, got %q", tok2)
	}

	// It shows up in the device list.
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].ID != d.ID {
		t.Fatalf("list: %+v err=%v", list, err)
	}

	// Revoking returns the session token (for the caller to delete) and removes
	// it from the list.
	rtok, err := s.Revoke(d.ID)
	if err != nil || rtok != "tok-123" {
		t.Fatalf("revoke: tok=%q err=%v", rtok, err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatalf("list after revoke should be empty, got %+v", list)
	}
}

func TestPairingPurgeExpired(t *testing.T) {
	s := newTestPairingStore(t)

	// One expired-pending, one live-pending, one approved.
	expired, _ := s.CreatePending("Old", "dock", -time.Minute)
	live, _ := s.CreatePending("Live", "dock", 10*time.Minute)
	approved, _ := s.CreatePending("Keep", "dock", 10*time.Minute)
	if _, err := s.Approve(approved.Code, "tok"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	n, err := s.PurgeExpired()
	if err != nil || n != 1 {
		t.Fatalf("purge: n=%d err=%v (want 1)", n, err)
	}
	// Expired pending is gone; live pending and approved survive.
	if _, err := s.GetByCode(expired.Code); err != ErrNotFound {
		t.Fatalf("expired should be purged, got %v", err)
	}
	if _, err := s.GetByCode(live.Code); err != nil {
		t.Fatalf("live pending should survive: %v", err)
	}
	if _, err := s.GetByCode(approved.Code); err != nil {
		t.Fatalf("approved should survive: %v", err)
	}
}

func TestPairingExpiredCodeCannotApprove(t *testing.T) {
	s := newTestPairingStore(t)
	d, err := s.CreatePending("Late Dock", "dock", -time.Minute) // already expired
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Approve(d.Code, "tok"); err != ErrNotFound {
		t.Fatalf("approve expired: want ErrNotFound, got %v", err)
	}
}
