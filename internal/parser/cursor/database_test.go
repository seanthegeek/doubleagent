package cursor

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The databases under testdata are written by SQLite from the rows in
// testdata/fixtures.py.

const stateRel = ".config/Cursor/User/globalStorage/state.vscdb"

// stateArtifact copies the named testdata database into a new home as its
// state.vscdb and returns the artifact.
func stateArtifact(t *testing.T, name string) *input.Artifact {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(stateRel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return &input.Artifact{DiskPath: path, Original: "/home/u/" + stateRel, Rel: stateRel, Agent: "cursor",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
}

// readState runs the primary store's reader over the named testdata
// database and returns its rows and the number of queries it ran.
func readState(t *testing.T, name string) ([]timeline.Row, int) {
	t.Helper()
	artifact := stateArtifact(t, name)
	database, err := sqlitefile.OpenPath(artifact.DiskPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := newStore(database, parser.CountingWork())
	var rows []timeline.Row
	New().state(artifact, store, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) })
	if store.err != nil {
		t.Fatal(store.err)
	}
	return rows, store.work.Count("queries")
}

func TestTheHeaderChunksAreReadOnceWhateverTheNumberOfComposers(t *testing.T) {
	_, few := readState(t, "empty_composers_2.db")
	_, many := readState(t, "empty_composers_8.db")
	if few == 0 || few != many {
		t.Errorf("2 composers ran %d queries and 8 ran %d; the count must not grow with the composers", few, many)
	}
}

func TestEachComposerGetsTheChunksUnderItsOwnKeyPrefix(t *testing.T) {
	rows, _ := readState(t, "chunked_headers.db")
	var got []string
	for _, row := range rows {
		got = append(got, row.SessionID+" "+row.Text)
	}
	// The key composerConvHeaders:a:b:000000 starts with a's prefix too, but
	// its composer is a:b, so a reads only its own chunk.
	want := []string{"a parser: missing bubble a1", "a:b parser: missing bubble ab1"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAHeaderWithAnEmptyComposerIDClaimsNoBlob(t *testing.T) {
	rows, _ := readState(t, "empty_composer_id.db")
	if len(rows) != 0 {
		t.Errorf("got %+v, want no rows: a header without an id is no conversation", rows)
	}
}

func TestAnMCPSecretsValueIsNeverSelected(t *testing.T) {
	artifact := stateArtifact(t, "mcp_secret.db")
	database, err := sqlitefile.OpenPath(artifact.DiskPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	table, err := database.Table("ItemTable")
	if err != nil {
		t.Fatal(err)
	}
	names, err := keyNames(table, func(row sqlitefile.Row) bool {
		return hasTextPrefix(row.Get("key"), mcpSecretPrefix)
	})
	if err != nil || len(names) != 1 || !names[0].value.IsNull() {
		t.Fatalf("got %+v, %v; want one key whose value is not read", names, err)
	}
	rows, _ := readState(t, "mcp_secret.db")
	want := "mcp server: srv url=https://tok:s3cret@mcp.example.invalid/ (from state.vscdb mcpOAuth.global, " +
		"file mtime)" + mcpTokens
	if len(rows) != 1 || rows[0].Text != want {
		t.Errorf("got %+v, want one row %q", rows, want)
	}
}

func TestAHeaderValueThatIsNotAnObjectIsReported(t *testing.T) {
	rows, _ := readState(t, "header_kinds.db")
	var got []string
	for _, row := range rows {
		if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
			got = append(got, row.SessionID+" "+row.Text)
		}
	}
	want := []string{"c1 parser: composerHeaders c1 is not a JSON object",
		"c2 parser: composerHeaders c2 is not a JSON object"}
	if !slices.Equal(got, want) {
		t.Errorf("problems %q, want %q", got, want)
	}
}
