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

// Note: ValidatePublicURL is intentionally NOT fuzzed — it performs a live DNS
// lookup (net.LookupIP), so it isn't a deterministic, side-effect-free target.
// Its URL/scheme/host validation is covered by the unit tests in unfurl_test.go.
