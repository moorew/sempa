package api

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
)

func newAuthTest(t *testing.T, cfg config.Config) *authHandler {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return newAuthHandler(cfg, conn)
}

func TestVerifyPassword_EnvBootstrapAdmin(t *testing.T) {
	h := newAuthTest(t, config.Config{AuthUsername: "admin", AuthPassword: "supersecret"})
	ctx := context.Background()

	email, userID, ok := h.verifyPassword(ctx, "admin", "supersecret")
	if !ok {
		t.Fatal("env bootstrap login should succeed")
	}
	if email != "admin" || userID == "" {
		t.Fatalf("expected admin email + an id, got %q / %q", email, userID)
	}
	// The bootstrap login must have provisioned a real admin user row.
	u, err := h.users.GetByID(ctx, userID)
	if err != nil || !u.IsAdmin {
		t.Fatalf("bootstrap user should exist and be admin: %v admin=%v", err, u.IsAdmin)
	}

	if _, _, ok := h.verifyPassword(ctx, "admin", "wrong"); ok {
		t.Fatal("wrong password must fail")
	}
}

func TestVerifyPassword_DBUserAndUnknown(t *testing.T) {
	h := newAuthTest(t, config.Config{AuthUsername: "admin", AuthPassword: "supersecret"})
	ctx := context.Background()

	hash, _ := db.HashPassword("wifepassword")
	if _, err := h.users.Create(ctx, db.CreateUserParams{
		Email: "Wife@Example.com", Name: "Wife", PasswordHash: &hash,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Case-insensitive email + correct password.
	email, userID, ok := h.verifyPassword(ctx, "wife@example.com", "wifepassword")
	if !ok || email != "wife@example.com" || userID == "" {
		t.Fatalf("db user login failed: ok=%v email=%q", ok, email)
	}
	// Wrong password for a real user.
	if _, _, ok := h.verifyPassword(ctx, "wife@example.com", "nope"); ok {
		t.Fatal("wrong password must fail")
	}
	// Unknown user — must fail without panicking (dummy-compare path).
	if _, _, ok := h.verifyPassword(ctx, "stranger@example.com", "whatever"); ok {
		t.Fatal("unknown user must fail")
	}
}

func TestUserStore_EnsureAndHashing(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	s := db.NewUserStore(conn)
	ctx := context.Background()

	u1, err := s.EnsureByEmail(ctx, "a@b.com", "A", true)
	if err != nil {
		t.Fatal(err)
	}
	u2, err := s.EnsureByEmail(ctx, "A@B.com", "ignored", false)
	if err != nil {
		t.Fatal(err)
	}
	if u1.ID != u2.ID {
		t.Fatal("EnsureByEmail must be idempotent on case-insensitive email")
	}
	if n, _ := s.Count(ctx); n != 1 {
		t.Fatalf("expected 1 user, got %d", n)
	}
	if !db.CheckPassword(mustHash(t, "pw1234567"), "pw1234567") {
		t.Fatal("bcrypt round-trip failed")
	}
	if db.CheckPassword(mustHash(t, "pw1234567"), "different") {
		t.Fatal("bcrypt must reject wrong password")
	}
}

func TestEmailOnAllowList_ClosedByDefault(t *testing.T) {
	// Empty list = nobody (closed by default — the Google gate relies on invites).
	empty := newAuthTest(t, config.Config{})
	if empty.emailOnAllowList("anyone@example.com") {
		t.Fatal("empty allow-list must NOT allow arbitrary emails")
	}
	// Non-empty list = explicit, case-insensitive membership only.
	listed := newAuthTest(t, config.Config{AllowedEmails: []string{"me@example.com"}})
	if !listed.emailOnAllowList("Me@Example.com") {
		t.Fatal("listed email should match case-insensitively")
	}
	if listed.emailOnAllowList("intruder@example.com") {
		t.Fatal("unlisted email must be rejected")
	}
}

// A Google invite is a passwordless DB user. It must be creatable and findable by
// email, since that's exactly how the Google sign-in gate recognises an invitee.
func TestInvite_PasswordlessUserIsFoundByEmail(t *testing.T) {
	h := newAuthTest(t, config.Config{})
	ctx := context.Background()

	invited, err := h.users.Create(ctx, db.CreateUserParams{Email: "Her@Example.com", Name: "Her"})
	if err != nil {
		t.Fatalf("invite create: %v", err)
	}
	if invited.HasPassword {
		t.Fatal("an invite must have no password")
	}
	// The gate looks the invitee up by (case-insensitive) email.
	found, err := h.users.GetByEmail(ctx, "her@example.com")
	if err != nil || found.ID != invited.ID {
		t.Fatalf("invited user must be found by email: %v", err)
	}
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := db.HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	return h
}
