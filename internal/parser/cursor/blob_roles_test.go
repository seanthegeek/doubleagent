package cursor

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// blobState returns a state.vscdb holding the header of the deleted
// composer gone and one agentKv blob naming it, whose JSON is given.
func blobState(t *testing.T, blob string) []byte {
	t.Helper()
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)"},
		sqlitetest.Table{Name: "cursorDiskKV", SQL: "CREATE TABLE cursorDiskKV (key TEXT, value BLOB)",
			Rows: [][]any{{"agentKv:blob:aa", blob}}},
		sqlitetest.Table{Name: "composerHeaders", SQL: "CREATE TABLE composerHeaders (composerId TEXT, " +
			"createdAt INTEGER, lastUpdatedAt INTEGER, isArchived INTEGER, value TEXT)",
			Rows: [][]any{{"gone", 1790000000000, 1790000000000, 1, `{"name":"n"}`}}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestABlobOfARoleOtherThanUserOrAssistantIsASystemRowLabelledByItsRole(t *testing.T) {
	tests := []struct {
		name, blob, want string
	}{
		{"a user message is a user row", `{"role":"user","content":"asked in gone"}`, "user asked in gone"},
		{"a system message is the system prompt", `{"role":"system","content":"rules for gone"}`,
			"system system prompt: rules for gone"},
		{"a message of another role names the role", `{"role":"tool","content":"output for gone"}`,
			"system tool message: output for gone"},
		{"a message with no role names none", `{"content":"text for gone"}`, "system - message: text for gone"},
		{"a message with an empty role names none", `{"role":"","content":"text for gone"}`,
			"system - message: text for gone"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, _ := parseStateOf(t, stateArtifactOf(t, blobState(t, test.blob)))
			timeline.Resolve(rows)
			// The blob's row is the composer's one row without a time.
			var got []string
			for _, row := range rows {
				if row.TimestampUTC == "" {
					got = append(got, row.TurnType+" "+row.Text)
				}
			}
			if !slices.Equal(got, []string{test.want}) {
				t.Errorf("rows %q, want %q", got, []string{test.want})
			}
		})
	}
}
