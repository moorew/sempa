package fastmail

import "testing"

// stripEmailPrefixes normalises subjects of imported (untrusted) emails into a
// task title; extractLinks pulls URLs out of the body. Both run on attacker-
// influenced input, so fuzz them for panics.
func FuzzStripEmailPrefixes(f *testing.F) {
	for _, s := range []string{
		"Re: Fwd: hello", "FWD: [EXT] news", "Re:Re:Re: x", "  spaced  ", "",
		"RE: [URGENT] ‮trick", "Fw: ", "AW: WG: subject",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_ = stripEmailPrefixes(s)
	})
}

func FuzzExtractLinks(f *testing.F) {
	for _, s := range []string{
		"see https://a.com and http://b.org/x?y=1", "no links here", "",
		"ftp://x https://ok.com", "https://", "http://a.com)trailing.",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_ = extractLinks(s)
	})
}
