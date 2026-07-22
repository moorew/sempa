package api

import "testing"

// Active/scriptable content must never be classified as inline — it has to
// download so it can't execute under the app origin. (AURA-SEC-002)
func TestIsSafeInlineMime(t *testing.T) {
	inline := []string{
		"image/png", "image/jpeg", "IMAGE/JPEG", "image/gif", "image/webp",
		"application/pdf", "text/plain", "text/plain; charset=utf-8",
		"text/csv", "video/mp4", "audio/mpeg",
	}
	download := []string{
		"text/html", "text/html; charset=utf-8", "image/svg+xml",
		"application/xhtml+xml", "application/xml", "text/xml",
		"application/javascript", "text/javascript", "application/octet-stream",
		"", "application/x-shockwave-flash",
	}
	for _, m := range inline {
		if !isSafeInlineMime(m) {
			t.Errorf("isSafeInlineMime(%q) = false, want true (inert, previewable)", m)
		}
	}
	for _, m := range download {
		if isSafeInlineMime(m) {
			t.Errorf("isSafeInlineMime(%q) = true, want false (active/unknown → download)", m)
		}
	}
}
