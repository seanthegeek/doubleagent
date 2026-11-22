package qwencode

import (
	"slices"
	"testing"
)

func TestLogsEntriesThatAreNotObjectsAreReportedOnce(t *testing.T) {
	const rel = ".qwen/tmp/h/logs.json"
	tests := []struct {
		name, text string
		want       []string
	}{
		{"three of four entries", `[1, "two", {"type": "user", "message": "hi"}, [3]]`,
			[]string{"3 hi", "0 parser: logs.json holds 3 entries that are not JSON objects"}},
		{"one entry", `[{"type": "user", "message": "hi"}, null]`,
			[]string{"1 hi", "0 parser: logs.json holds 1 entry that is not a JSON object"}},
		{"entries before a cut", `[7, {"type": "user", "message": "hi"}`,
			[]string{"2 hi", "3 parser: unterminated JSON array", "0 parser: logs.json holds 1 entry that is not a JSON object"}},
		{"objects only", `[{"type": "user", "message": "hi"}]`, []string{"1 hi"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parsedRows(t, rel, test.text); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
