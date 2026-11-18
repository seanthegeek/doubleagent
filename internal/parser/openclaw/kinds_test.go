package openclaw

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// execProblems runs execDocument on the approvals file text, a stored row
// with rowid 4, and returns the texts of the problems it adds. The
// approvals file is the SQLite column's text, which only a Python
// generator could put in a database, so the function is called directly.
func execProblems(t *testing.T, text string) []string {
	t.Helper()
	document, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	rows := &stateRows{Parser: New(), emit: func(timeline.Row) {}}
	rows.execDocument(4, document, "")
	var texts []string
	for _, found := range rows.problems {
		if found.Line != 4 {
			t.Errorf("problem %q at line %d, want 4", found.Message, found.Line)
		}
		texts = append(texts, found.Message)
	}
	return texts
}

func TestAnExecApprovalsMemberOfTheWrongKindIsReported(t *testing.T) {
	array, object := jsonvalue.Array, jsonvalue.Object
	tests := []struct {
		name, member string
		kind         jsonvalue.Kind
		other        string
		build        func(value string) string
	}{
		{"agents", "agents", object, `[1]`, func(text string) string { return `{"agents": ` + text + `}` }},
		{"defaults", "defaults", object, `[1]`, func(text string) string { return `{"defaults": ` + text + `}` }},
		{"an agent", "agents.main", object, `[1]`, func(text string) string { return `{"agents": {"main": ` + text + `}}` }},
		{"allowlist", "defaults.allowlist", array, `{"pattern": "ls"}`,
			func(text string) string { return `{"defaults": {"allowlist": ` + text + `}}` }},
		{"mcpTools", "agents.main.mcpTools", array, `{"server": "s"}`,
			func(text string) string { return `{"agents": {"main": {"mcpTools": ` + text + `}}}` }},
	}
	for _, test := range tests {
		problem := parser.WrongKind(test.member, test.kind)
		empty := `[]`
		if test.kind == object {
			empty = `{}`
		}
		for _, value := range []struct {
			name, json string
			reported   bool
		}{{"a string", `"oops"`, true}, {"a number", `7`, true}, {"the other kind", test.other, true},
			{"null", `null`, false}, {"empty", empty, false}} {
			t.Run(test.name+" "+value.name, func(t *testing.T) {
				want := []string(nil)
				if value.reported {
					want = []string{problem}
				}
				if got := execProblems(t, test.build(value.json)); !slices.Equal(got, want) {
					t.Errorf("problems %q, want %q", got, want)
				}
			})
		}
	}
	t.Run("the file itself", func(t *testing.T) {
		if got, want := execProblems(t, `[1]`), []string{parser.WrongKind("raw_json", object)}; !slices.Equal(got, want) {
			t.Errorf("problems %q, want %q", got, want)
		}
	})
	t.Run("absent members", func(t *testing.T) {
		if got := execProblems(t, `{"version": 1}`); len(got) != 0 {
			t.Errorf("problems %q, want none", got)
		}
	})
}
