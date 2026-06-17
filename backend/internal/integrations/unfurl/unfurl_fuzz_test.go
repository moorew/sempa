package unfurl

import (
	"net/url"
	"testing"
)

// parseMeta runs over HTML fetched from arbitrary (user-supplied) URLs, so it
// must tolerate any markup without panicking.
func FuzzParseMeta(f *testing.F) {
	f.Add(`<html><head><meta property="og:title" content="X"><title>T</title></head></html>`)
	f.Add(`<meta property=og:image content=//host/x.png>`)
	f.Add(`<meta name="twitter:image" content="/a.png">`)
	f.Add("")
	base, _ := url.Parse("https://example.com/page")
	f.Fuzz(func(t *testing.T, doc string) {
		_ = parseMeta(doc, base)
	})
}

// ValidatePublicURL is the SSRF gate for the unfurl/link-preview fetcher; it
// must reject (not crash on) any malformed or internal URL.
func FuzzValidatePublicURL(f *testing.F) {
	for _, s := range []string{
		"https://example.com", "http://localhost:8080/x", "ftp://example.com",
		"http://169.254.169.254/latest", "http://[::1]/", "", "https://",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		_, _ = ValidatePublicURL(raw)
	})
}
