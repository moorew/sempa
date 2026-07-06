package unfurl

import (
	"net"
	"testing"
)

// isPrivateIP is the SSRF classifier behind ValidatePublicURL. ValidatePublicURL
// itself does a live DNS lookup (so it's covered by the unit tests in
// unfurl_test.go and is not a fuzz target), but the IP-range logic is pure and
// security-critical, so pin it down here — including the cloud metadata address.
func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},             // loopback
		{"::1", true},                   // loopback v6
		{"10.0.0.1", true},              // RFC1918
		{"172.16.5.4", true},            // RFC1918
		{"192.168.1.1", true},           // RFC1918
		{"169.254.169.254", true},       // link-local (cloud metadata)
		{"0.0.0.0", true},               // unspecified
		{"fd00::1", true},               // IPv6 unique-local
		{"fe80::1", true},               // IPv6 link-local
		{"8.8.8.8", false},              // public
		{"1.1.1.1", false},              // public
		{"2606:4700:4700::1111", false}, // public v6
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", c.ip)
		}
		if got := isPrivateIP(ip); got != c.private {
			t.Errorf("isPrivateIP(%s) = %v, want %v", c.ip, got, c.private)
		}
	}
}

// safeDialControl is the connect-time guard that closes the DNS-rebinding gap:
// it runs with the resolved ip:port actually being dialed, so a hostname that
// passed ValidatePublicURL but re-resolves to a private IP is still refused.
func TestSafeDialControl(t *testing.T) {
	cases := []struct {
		address string
		reject  bool
	}{
		{"127.0.0.1:80", true},                // loopback
		{"169.254.169.254:80", true},          // cloud metadata
		{"10.1.2.3:443", true},                // RFC1918
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
			t.Errorf("safeDialControl(%q) = %v, want allowed", c.address, err)
		}
	}
}
