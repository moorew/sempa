package unfurl

import "testing"

func TestInertImageType(t *testing.T) {
	for _, ct := range []string{"image/png", "image/jpeg; charset=binary", "IMAGE/WEBP", "image/x-icon"} {
		if !inertImageType(ct) {
			t.Errorf("%q should be allowed", ct)
		}
	}
	for _, ct := range []string{"image/svg+xml", "image/svg+xml; charset=utf-8", "text/html", "image/", "", "application/xml"} {
		if inertImageType(ct) {
			t.Errorf("%q must be refused", ct)
		}
	}
}
