package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimiter is a tiny in-memory fixed-window per-IP limiter for public,
// row-creating endpoints (e.g. /devices/pair/start) so an unauthenticated caller
// can't flood the server. Dependency-free; fine for a self-hosted single-user
// deployment. Not a distributed limiter.
type rateLimiter struct {
	max    int
	window time.Duration
	mu     sync.Mutex
	hits   map[string]*ipWindow
}

type ipWindow struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, hits: make(map[string]*ipWindow)}
}

func clientIP(r *http.Request) string {
	// chi's RealIP middleware already normalizes RemoteAddr from X-Forwarded-For.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func (rl *rateLimiter) allow(ip string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	w := rl.hits[ip]
	if w == nil || now.After(w.resetAt) {
		// New window. Opportunistically drop other expired entries so the map
		// doesn't grow unbounded across many IPs.
		if len(rl.hits) > 256 {
			for k, v := range rl.hits {
				if now.After(v.resetAt) {
					delete(rl.hits, k)
				}
			}
		}
		rl.hits[ip] = &ipWindow{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	if w.count >= rl.max {
		return false
	}
	w.count++
	return true
}

// middleware returns a chi-compatible middleware enforcing the limit.
func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r), time.Now()) {
			respondError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
