package growthtests

import (
	"maps"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// stateItem returns a state.vscdb whose ItemTable holds value under key.
func stateItem(t *testing.T, key, value string) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)",
		Rows: [][]any{{key, value}}})
}

func TestAStateDatabaseItemThatRepeatsANameIsReportedOnTheDatabase(t *testing.T) {
	tests := []struct {
		parser, agent, rel, key string
	}{
		{"cody", codyEditorAgent, codyState, "sourcegraph.cody-ai"},
		{"twinny", twinnyEditor, twinnyState, "rjmacarthy.twinny"},
		{"tabby", "vscode", ".config/Code/User/globalStorage/state.vscdb", "TabbyML.vscode-tabby"},
		{"pearai", "", pearaiState, "pearai.pearai-roo-cline"},
	}
	for _, test := range tests {
		t.Run(test.parser, func(t *testing.T) {
			files := one(test.rel, stateItem(t, test.key, `{"x":1,"y":{},"x":2}`))
			got := repeatRowsByFileOf(t, test.parser, test.agent, files)
			if want := map[string]int{"/home/u/" + test.rel: 1}; !maps.Equal(got, want) {
				t.Errorf("repeat rows by file %v, want %v", got, want)
			}
		})
	}
}
