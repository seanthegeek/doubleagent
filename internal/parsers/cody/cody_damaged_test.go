package cody

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// damagedStateDatabase returns a state.vscdb whose sourcegraph.cody-ai row
// holds value.
func damagedStateDatabase(t *testing.T, value string) []byte {
	t.Helper()
	data, err := sqlitetest.Build(sqlitetest.Table{Name: "ItemTable",
		SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)", Rows: [][]any{{itemKey, value}}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Every path that recovers a damaged history gives one problem row saying
// its typed prompts were not read: the row value that is not JSON (whose
// reason ReadItem's row gives), a history member kept as JSON text and the
// JetBrains file.
func TestEveryDamagedHistorySaysItsTypedPromptsWereNotRead(t *testing.T) {
	t.Parallel()
	const date = "Mon, 01 Jan 2024 10:00:00 GMT"
	history := `{"https://x.example-u":{"chat":{"` + date + `":{"id":"` + date + `","interactions":[` +
		`{"humanMessage":{"speaker":"human","text":"kept"}}]}},"input":["kept","lost"]}`
	cut := strings.TrimSuffix(history, "}")
	const clause = "parser: chat history did not parse; recovered 1 chat(s), typed prompts not read"
	tests := []struct {
		name, rel string
		data      func(t *testing.T) []byte
		want      string
	}{
		{"the row value", "Code/User/globalStorage/state.vscdb", func(t *testing.T) []byte {
			return damagedStateDatabase(t, `{"`+historyKey+`":`+cut)
		}, clause},
		{"a history member kept as text", "Code/User/globalStorage/state.vscdb", func(t *testing.T) []byte {
			return damagedStateDatabase(t, `{"`+historyKey+`":"`+strings.ReplaceAll(cut, `"`, `\"`)+`"}`)
		}, clause + " ("},
		{"the JetBrains file", "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2",
			func(t *testing.T) []byte { return []byte(cut) }, clause + " ("},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := parseFile(t, test.rel, test.data(t))
			var clauses []string
			for _, text := range got {
				if strings.Contains(text, "typed prompts not read") {
					clauses = append(clauses, text)
				}
			}
			if len(clauses) != 1 || !strings.HasPrefix(clauses[0], test.want) {
				t.Errorf("rows %q, want one row starting %q", got, test.want)
			}
			if test.want == clause && !slices.Contains(got, clause) {
				t.Errorf("rows %q, want the row %q with no reason after it", got, clause)
			}
			if !slices.Contains(got, "kept") || slices.Contains(got, "lost") {
				t.Errorf("rows %q, want the chat's prompt and no typed-prompt row", got)
			}
		})
	}
}
