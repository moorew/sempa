package backup

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/clevercode/sempa/internal/db"
)

// No bundle, in any mode, may carry a live login token, and a restore must not
// bring one back.
func TestSnapshotDropsSessions(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{ModeNone, ModeExcludeSecrets, ModeEncrypt} {
		p := filepath.Join(t.TempDir(), "snap.db")
		conn, err := db.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Migrate(conn); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(`INSERT INTO sessions (id, email, user_id, expires_at, scope) VALUES ('tok', 'a@example.test', 'u1', '2999-01-01T00:00:00Z', 'full')`); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(`INSERT INTO paired_devices (id, code, device_name, platform, status, token, expires_at, created_at) VALUES ('d1', 'C', 'dock', 'pi', 'approved', 'devtok', '2999-01-01', '2026-01-01')`); err != nil {
			t.Fatal(err)
		}
		conn.Close()

		if err := sanitizeSnapshot(ctx, p, mode); err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		check, _ := sql.Open("sqlite", p)
		var n int
		_ = check.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n)
		var tok sql.NullString
		if err := check.QueryRow(`SELECT token FROM paired_devices WHERE id = 'd1'`).Scan(&tok); err != nil {
			t.Fatalf("%s: paired device row: %v", mode, err)
		}
		check.Close()
		if n != 0 || tok.Valid {
			t.Errorf("%s: sessions=%d device token kept=%v", mode, n, tok.Valid)
		}
	}
	if !restoreSkip["sessions"] {
		t.Error("restore would bring back the bundle's sessions")
	}
}
