package ai

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain object", `{"a":1}`, `{"a":1}`},
		{"prose wrapper", "Sure! Here you go:\n{\"a\":1}\nHope that helps.", `{"a":1}`},
		{"array", `[1,2,3]`, `[1,2,3]`},
		{
			// The reason for tracking string state: braces inside a string value
			// must not be counted as structural, or the object truncates early.
			"braces inside string value",
			`{"title":"fix render() {}","ok":true}`,
			`{"title":"fix render() {}","ok":true}`,
		},
		{
			"escaped quote inside string",
			`{"t":"a \" {brace} still inside","n":1}`,
			`{"t":"a \" {brace} still inside","n":1}`,
		},
		{"no json", `nothing here`, `nothing here`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractJSON(c.in); got != c.want {
				t.Errorf("extractJSON(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// extractJSON runs over whatever the local model emits (chatty prose, partial
// objects, braces inside strings). Fuzz it to be sure no output can panic the
// string-state scanner.
func FuzzExtractJSON(f *testing.F) {
	for _, s := range []string{
		`{"a":1}`, "prose {\"a\":1} more", "[1,2,3]", `{"t":"x {} y"}`,
		`{"t":"a \" {brace}"}`, "{unbalanced", "no json", "", `[{"k":"]"}]`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_ = extractJSON(s)
	})
}
