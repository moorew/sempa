package gmail

import "testing"

// A Gmail-flow state must not finish the Drive backup flow (or vice versa).
func TestStateBoundToPurpose(t *testing.T) {
	s := GenerateStateFor(PurposeGmail, "user-1")
	if _, ok := ConsumeStateFor(s, PurposeDriveBackup); ok {
		t.Fatal("gmail state accepted by the drive flow")
	}
	s = GenerateStateFor(PurposeDriveBackup, "admin-1")
	uid, ok := ConsumeStateFor(s, PurposeDriveBackup)
	if !ok || uid != "admin-1" {
		t.Fatalf("drive state rejected: ok=%v uid=%q", ok, uid)
	}
	if _, ok := ConsumeStateFor(s, PurposeDriveBackup); ok {
		t.Fatal("state reusable")
	}
}
