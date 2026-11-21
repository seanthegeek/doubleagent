package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// ParseJSONC and StripJSONC followed by a decode read JSON with comments
// alike, except for a leading byte-order mark.
func TestJSONCReadsCommentsAndTrailingCommasOutsideStrings(t *testing.T) {
	tests := []struct {
		name, raw, parsed, stripped string
	}{
		{"line and block comments and trailing commas go",
			"{\"a\": 1, // c\n \"b\": \"http://x\", /* x */ \"c\": [1, 2,],}",
			`{"a":1,"b":"http://x","c":[1,2]}`, `{"a":1,"b":"http://x","c":[1,2]}`},
		{"a byte-order mark is dropped only by the parser", "\ufeff{\"a\":1}", `{"a":1}`, "error"},
		{"an escaped quote keeps a string open", `{"a":"\"//"} // tail`, `{"a":"\"//"}`,
			`{"a":"\"//"}`},
		{"an unclosed block comment runs to the end", `{"a":1} /* open`, `{"a":1}`, `{"a":1}`},
		{"a comma at the end of the text stays", `{"a":1},`, "error", "error"},
		{"a comment between a comma and a bracket", `[1, /* c */ ]`, `[1]`, `[1]`},
		{"an escaped backslash ends before the quote", "{\"a\":\"\\\\\"}//x\n", `{"a":"\\"}`,
			`{"a":"\\"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := compactOrError(ParseJSONC(nil, []byte(test.raw))); got != test.parsed {
				t.Errorf("ParseJSONC: got %s, want %s", got, test.parsed)
			}
			if got := compactOrError(jsonvalue.DecodeString(StripJSONC(test.raw))); got != test.stripped {
				t.Errorf("StripJSONC: got %s, want %s", got, test.stripped)
			}
		})
	}
}

// compactOrError returns a decoded value as compact JSON, or "error".
func compactOrError(value jsonvalue.Value, err error) string {
	if err != nil {
		return "error"
	}
	return value.JSON()
}

func FuzzParseJSONC(fuzz *testing.F) {
	for _, seed := range []string{`{"a": 1, // c` + "\n}", `[1, /* c */ ]`, `"\"//`, `/*`, `,]`} {
		fuzz.Add([]byte(seed))
	}
	fuzz.Fuzz(func(t *testing.T, raw []byte) {
		stripped := StripJSONC(string(raw))
		if len(stripped) > len(raw) {
			t.Errorf("stripping grew %d bytes to %d", len(raw), len(stripped))
		}
		_, _ = ParseJSONC(nil, raw)
	})
}
