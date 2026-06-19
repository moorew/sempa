package api

import (
	"testing"
	"time"
)

func TestRateLimiterFixedWindow(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	base := time.Now()

	// First 3 in the window pass; the 4th is blocked.
	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4", base) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.allow("1.2.3.4", base) {
		t.Fatal("4th request in window should be blocked")
	}

	// A different IP has its own budget.
	if !rl.allow("5.6.7.8", base) {
		t.Fatal("different IP should be allowed")
	}

	// After the window rolls over, the original IP is allowed again.
	if !rl.allow("1.2.3.4", base.Add(time.Minute+time.Second)) {
		t.Fatal("request after window reset should be allowed")
	}
}
