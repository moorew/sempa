package netguard

import (
	"net"
	"testing"
)

func TestIsNonPublic(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.83", "169.254.169.254",
		"100.64.0.1", "100.100.100.100", "100.127.255.254", "0.0.0.1", "198.18.0.1",
		"240.0.0.1", "255.255.255.255", "192.0.0.1", "::1", "fd7a:115c:a1e0::1",
		"fe80::1", "64:ff9b::a9fe:a9fe", "::ffff:127.0.0.1", "::ffff:100.64.0.1", "::",
	}
	for _, s := range blocked {
		if !IsNonPublic(net.ParseIP(s)) {
			t.Errorf("%s should be non-public", s)
		}
	}
	allowed := []string{"1.1.1.1", "8.8.8.8", "100.63.255.255", "100.128.0.1", "2606:4700::1111"}
	for _, s := range allowed {
		if IsNonPublic(net.ParseIP(s)) {
			t.Errorf("%s should be public", s)
		}
	}
}
