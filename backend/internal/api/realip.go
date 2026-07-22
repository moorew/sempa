package api

import (
	"net"
	"net/http"
	"strings"
)

// defaultTrustedProxyCIDRs are the peer ranges whose forwarding headers we trust
// by default: loopback, RFC1918 private, CGNAT/Tailscale (100.64/10) and IPv6
// unique-local / link-local. A reverse proxy fronting the app almost always
// connects from one of these; a directly connected public client never does, so
// it cannot spoof X-Forwarded-For / X-Real-IP to rotate its apparent IP.
var defaultTrustedProxyCIDRs = []string{
	"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"100.64.0.0/10", "169.254.0.0/16", "::1/128", "fc00::/7", "fe80::/10",
}

func parseCIDRs(cidrs []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// Accept a bare IP as well as a CIDR.
		if !strings.Contains(c, "/") {
			if ip := net.ParseIP(c); ip != nil {
				if ip.To4() != nil {
					c += "/32"
				} else {
					c += "/128"
				}
			}
		}
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// realIP returns a middleware that rewrites r.RemoteAddr from the forwarding
// headers ONLY when the socket peer is itself a trusted proxy. Unlike chi's
// middleware.RealIP — which trusts X-Forwarded-For / X-Real-IP unconditionally —
// this prevents a directly connected client from forging those headers to rotate
// its apparent IP and evade the login/pairing throttles. (AURA-SEC-004)
func realIP(trusted []string) func(http.Handler) http.Handler {
	nets := parseCIDRs(trusted)
	if len(nets) == 0 {
		nets = parseCIDRs(defaultTrustedProxyCIDRs)
	}
	trustedPeer := func(addr string) bool {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		ip := net.ParseIP(strings.TrimSpace(host))
		return ip != nil && ipInNets(ip, nets)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trustedPeer(r.RemoteAddr) {
				if ip := forwardedForClientIP(r, nets); ip != "" {
					if _, port, err := net.SplitHostPort(r.RemoteAddr); err == nil {
						r.RemoteAddr = net.JoinHostPort(ip, port)
					} else {
						r.RemoteAddr = net.JoinHostPort(ip, "0")
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// forwardedForClientIP resolves the real client IP from a trusted-proxy request.
// It walks X-Forwarded-For right-to-left and returns the first address that is
// NOT itself a trusted proxy — the client as seen at our trust boundary. This
// defeats a client that prepends a spoofed entry, because the honest proxy
// appends the true peer to the right. Falls back to X-Real-IP, then the
// left-most valid entry. Returns "" when nothing usable is present.
func forwardedForClientIP(r *http.Request, trusted []*net.IPNet) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(xr) != nil {
			return xr
		}
		return ""
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		p := net.ParseIP(ip)
		if p == nil {
			continue
		}
		if !ipInNets(p, trusted) {
			return ip
		}
	}
	// Every hop was a trusted proxy — fall back to the left-most valid address.
	for _, seg := range parts {
		if ip := strings.TrimSpace(seg); net.ParseIP(ip) != nil {
			return ip
		}
	}
	return ""
}
