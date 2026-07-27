package db

import (
	"context"
	"testing"
)

// The reason code exists so the UI can distinguish "backups are broken and only
// YOU can fix it" (expired Google token — every nightly run will keep failing)
// from ordinary transient failures, without string-matching a human message.
func TestRecordResultRoundTripsErrorCode(t *testing.T) {
	dbConn := newTestStore(t).db
	s := NewBackupStore(dbConn)
	ctx := context.Background()

	msg := "Google authorization expired — please reconnect the account"
	if err := s.RecordResult(ctx, "error", &msg, BackupErrReauthRequired); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastErrorCode == nil || *got.LastErrorCode != BackupErrReauthRequired {
		t.Fatalf("expected %q, got %v", BackupErrReauthRequired, got.LastErrorCode)
	}
	if got.LastStatus == nil || *got.LastStatus != "error" {
		t.Fatalf("expected last_status=error, got %v", got.LastStatus)
	}

	// A later success must clear the code, or the banner would nag forever after
	// the user reconnects.
	if err := s.RecordResult(ctx, "success", nil, ""); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastErrorCode != nil {
		t.Fatalf("a successful run must clear the error code, got %v", *got.LastErrorCode)
	}
	if got.LastError != nil {
		t.Fatalf("a successful run must clear the error message, got %v", *got.LastError)
	}
}
