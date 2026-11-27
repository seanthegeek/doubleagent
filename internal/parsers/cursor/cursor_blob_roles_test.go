package cursor

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// blobState returns a state.vscdb holding the headers of the deleted
// composers gone and also, and one agentKv blob naming both, whose JSON
// is given.
func blobState(t *testing.T, blob string) []byte {
	t.Helper()
	header := func(ident string) []any { return []any{ident, 1790000000000, 1790000000000, 1, `{"name":"n"}`} }
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)"},
		sqlitetest.Table{Name: "cursorDiskKV", SQL: "CREATE TABLE cursorDiskKV (key TEXT, value BLOB)",
			Rows: [][]any{{"agentKv:blob:aa", blob}}},
		sqlitetest.Table{Name: "composerHeaders", SQL: "CREATE TABLE composerHeaders (composerId TEXT, " +
			"createdAt INTEGER, lastUpdatedAt INTEGER, isArchived INTEGER, value TEXT)",
			Rows: [][]any{header("gone"), header("also")}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// blobRows returns, as "session type text", the rows the blob of the
// state.vscdb above gives (the composers' only rows without a time) and
// its problem rows, each shared text written out in its place.
func blobRows(t *testing.T, blob string) []string {
	t.Helper()
	rows, _ := parseStateOf(t, stateArtifactOf(t, blobState(t, blob)))
	timeline.Resolve(rows)
	var got []string
	for _, row := range rows {
		if row.TimestampUTC == "" {
			got = append(got, row.SessionID+" "+row.TurnType+" "+row.Text)
		}
	}
	return got
}

func TestABlobsRowsAreLabelledByItsRoleAndALaterRowPointsAtTheFirst(t *testing.T) {
	tests := []struct {
		name, blob, label, turnType string
	}{
		{"a user message is a user row", `{"role":"user","content":"asked in gone also"}`, "", "user"},
		{"an assistant message is an assistant row", `{"role":"assistant","content":"asked in gone also"}`, "",
			"assistant"},
		{"a system message is the system prompt", `{"role":"system","content":"asked in gone also"}`,
			"system prompt: ", "system"},
		{"a message of another role names the role", `{"role":"tool","content":"asked in gone also"}`,
			"tool message: ", "system"},
		{"a message with no role names none", `{"content":"asked in gone also"}`, "- message: ", "system"},
		{"a message with an empty role names none", `{"role":"","content":"asked in gone also"}`, "- message: ",
			"system"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := []string{"gone " + test.turnType + " " + test.label + "asked in gone also",
				"also " + test.turnType + " " + test.label + blobPointerText(1)}
			if got := blobRows(t, test.blob); !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
}

// A role longer than the carried bound is cut once for the blob, so each
// composer's row holds the cut label and one problem row counts the cut.
func TestALongBlobRoleIsCutOnceForAllTheComposersRows(t *testing.T) {
	role := strings.Repeat("r", 2*parserkit.CarriedLimit)
	got := blobRows(t, `{"role":"`+role+`","content":"asked in gone also"}`)
	if len(got) != 3 {
		t.Fatalf("rows %q, want two blob rows and a problem row", got)
	}
	for _, row := range got[:2] {
		label := row[strings.Index(row, "system ")+len("system "):]
		label = label[:strings.Index(label, " message: ")]
		if len(label) > parserkit.CarriedLimit || !strings.HasSuffix(label, " [cut from 8192 bytes]") {
			t.Errorf("label of %d bytes ending %q, want one cut to %d bytes", len(label), label[max(0, len(label)-30):],
				parserkit.CarriedLimit)
		}
	}
	if want := " system parser: " + parserkit.CarriedProblem("agentKv blob role", 1); got[2] != want {
		t.Errorf("problem row %q, want %q", got[2], want)
	}
}
