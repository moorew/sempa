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
