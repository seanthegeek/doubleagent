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

// Every path that reads a history chat by chat gives one problem row saying
// its typed prompts were not read: a row value that is not JSON (beside
// ReadItem's row, which gives the reason), a row value that is a JSON
// string (with no such row), a history member kept as JSON text and the
// JetBrains file (each with the reason after the clause).
func TestEveryDamagedHistorySaysItsTypedPromptsWereNotRead(t *testing.T) {
	t.Parallel()
	const date = "Mon, 01 Jan 2024 10:00:00 GMT"
	history := `{"https://x.example-u":{"chat":{"` + date + `":{"id":"` + date + `","interactions":[` +
		`{"humanMessage":{"speaker":"human","text":"kept"}}]}},"input":["kept","lost"]}`
	cut := strings.TrimSuffix(history, "}")
	const clause = "parser: chat history did not parse; recovered 1 chat(s), typed prompts not read"
	const state = "Code/User/globalStorage/state.vscdb"
	tests := []struct {
		name, rel     string
		data          func(t *testing.T) []byte
		want          string
		itemRow, chat bool // ReadItem's not-valid-JSON row, and the recovered chat's row
	}{
		{"a row value that is not JSON", state, func(t *testing.T) []byte {
			return damagedStateDatabase(t, `{"`+historyKey+`":`+cut)
		}, clause, true, true},
		{"a row value that is a JSON string", state, func(t *testing.T) []byte {
			return damagedStateDatabase(t, `"abc"`)
		}, "parser: chat history did not parse; recovered 0 chat(s), typed prompts not read", false, false},
		{"a history member kept as text", state, func(t *testing.T) []byte {
			return damagedStateDatabase(t, `{"`+historyKey+`":"`+strings.ReplaceAll(cut, `"`, `\"`)+`"}`)
		}, clause + " (", false, true},
		{"the JetBrains file", "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2",
			func(t *testing.T) []byte { return []byte(cut) }, clause + " (", false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := parseFile(t, test.rel, test.data(t))
			var clauses []string
			itemRow := false
			for _, text := range got {
				if strings.Contains(text, "typed prompts not read") {
					clauses = append(clauses, text)
				}
				itemRow = itemRow || strings.HasPrefix(text, "parser: ItemTable value for "+itemKey+" is not valid JSON (")
			}
			if len(clauses) != 1 || !strings.HasPrefix(clauses[0], test.want) {
				t.Errorf("rows %q, want one row starting %q", got, test.want)
			}
			if !strings.HasSuffix(test.want, " (") && !slices.Contains(got, test.want) {
				t.Errorf("rows %q, want the row %q with no reason after it", got, test.want)
			}
			if itemRow != test.itemRow {
				t.Errorf("rows %q: the not-valid-JSON row is there %v, want %v", got, itemRow, test.itemRow)
			}
			if slices.Contains(got, "kept") != test.chat || slices.Contains(got, "lost") {
				t.Errorf("rows %q, want the chat's prompt %v and no typed-prompt row", got, test.chat)
			}
		})
	}
}
