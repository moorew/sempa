package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A client connecting directly (public peer) must not be able to spoof its IP
// via X-Forwarded-For / X-Real-IP; a request arriving from a trusted proxy peer
// should have those headers honoured. (AURA-SEC-004)
func TestRealIPTrustBoundary(t *testing.T) {
	mw := realIP(nil) // nil → default trusted set (loopback + private/ULA/CGNAT)

	capture := func(remoteAddr string, headers map[string]string) string {
		var seen string
		h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			seen = clientIP(r)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remoteAddr
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		return seen
	}

	cases := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "direct public client cannot spoof XFF",
			remoteAddr: "203.0.113.7:5555",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4"},
			want:       "203.0.113.7",
		},
		{
			name:       "direct public client cannot spoof X-Real-IP",
			remoteAddr: "203.0.113.7:5555",
			headers:    map[string]string{"X-Real-IP": "1.2.3.4"},
			want:       "203.0.113.7",
		},
		{
			name:       "trusted loopback proxy: XFF honoured",
			remoteAddr: "127.0.0.1:5555",
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.9"},
			want:       "198.51.100.9",
		},
		{
			name:       "trusted proxy with prepended spoof: right-most untrusted wins",
			remoteAddr: "127.0.0.1:5555",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4, 198.51.100.9"},
			want:       "198.51.100.9",
		},
		{
			name:       "no forwarding headers: socket peer used",
			remoteAddr: "203.0.113.7:5555",
			want:       "203.0.113.7",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := capture(c.remoteAddr, c.headers); got != c.want {
				t.Errorf("clientIP = %q, want %q", got, c.want)
			}
		})
	}
}

// An explicit trusted-proxy allowlist should NOT trust an arbitrary private peer
// that isn't in it — proving the config is honoured rather than always falling
// back to the private-range default.
func TestRealIPExplicitAllowlist(t *testing.T) {
	mw := realIP([]string{"192.0.2.10/32"})
	var seen string
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = clientIP(r) }))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:4444" // private, but not the configured proxy
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "10.0.0.5" {
		t.Errorf("clientIP = %q, want 10.0.0.5 (untrusted peer, XFF ignored)", seen)
	}
}
