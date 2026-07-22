package ical

import "testing"

// safeDialControl is the connect-time SSRF guard for ICS feed fetches. It runs
// with the resolved ip:port actually being dialed, so a hostname that passed
// validateURL but re-resolves to a private IP (DNS rebinding) is still refused.
// Mirrors the unfurl package's guard. (AURA-SEC-003)
func TestSafeDialControl(t *testing.T) {
	cases := []struct {
		address string
		reject  bool
	}{
		{"127.0.0.1:80", true},                // loopback
		{"169.254.169.254:80", true},          // cloud metadata
		{"10.1.2.3:443", true},                // RFC1918
		{"192.168.0.9:80", true},              // RFC1918
		{"[::1]:80", true},                    // loopback v6
		{"[fd00::1]:443", true},               // IPv6 unique-local
		{"8.8.8.8:443", false},                // public
		{"[2606:4700:4700::1111]:443", false}, // public v6
	}
	for _, c := range cases {
		err := safeDialControl("tcp", c.address, nil)
		if c.reject && err == nil {
			t.Errorf("safeDialControl(%q) = nil, want rejection", c.address)
		}
		if !c.reject && err != nil {
			t.Errorf("safeDialControl(%q) = %v, want allow", c.address, err)
		}
	}
}
