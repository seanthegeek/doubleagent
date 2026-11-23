package chatgptdesktop

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The conversation rows themselves are checked by the golden output of the
// analysis commands (internal/analyze), over inputs that hold every parsed
// agent; these tests cover what needs no database built by SQLite.

func TestWantsOnlyTheAppDatabaseAtTheHome(t *testing.T) {
	tests := []struct {
		rel  string
		want bool
	}{
		{".codex/sqlite/codex-dev.db", true},
		{".codex/sqlite/codex-dev.db-wal", false},
		{".codex/sqlite/codex-dev.db-shm", false},
		{".codex/config.toml", false},
		{".codex/state_5.sqlite", false},
		{"proj/.codex/sqlite/codex-dev.db", false},
		{".openinterpreter/sqlite/codex-dev.db", false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			if got := New().Wants(&input.Artifact{Rel: test.rel}); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestADatabaseThatCannotBeReadIsOneProblemRow(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		absent bool
		want   string
	}{
		{"a text file", []byte("not sqlite"), false, "parser: not a SQLite database"},
		{"a header over a damaged body", append([]byte("SQLite format 3\x00"),
			slices.Repeat([]byte{0xff}, 200)...), false, "parser: SQLite error: "},
		{"an empty database has no table", nil, false, "parser: no local_thread_catalog table"},
		{"a missing file", nil, true,
			"parser: codex-dev.db could not be read (no such file or directory)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "codex-dev.db")
			if !test.absent {
				if err := osfile.WriteFile(path, test.data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/alice/" + databaseRel,
				Rel: databaseRel, Agent: "codex-cli", Home: &input.Home{Host: "h1", User: "alice"}}
			var rows []timeline.Row
			err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) })
			if err != nil || len(rows) != 1 || !strings.HasPrefix(rows[0].Text, test.want) {
				t.Fatalf("got %+v, %v; want one row starting %q", rows, err, test.want)
			}
			want := timeline.Row{Host: "h1", User: "alice", Agent: "chatgpt-desktop",
				TurnType: "system", Text: rows[0].Text, SourceFile: "/alice/" + databaseRel}
			if rows[0] != want {
				t.Errorf("got %+v, want %+v", rows[0], want)
			}
		})
	}
}

func TestTheParserIsOfferedCodexFilesAndRecordsTwoMarks(t *testing.T) {
	subject := New()
	if subject.Agent() != "chatgpt-desktop" || subject.Name() != "chatgpt-desktop" {
		t.Errorf("agent %q, name %q", subject.Agent(), subject.Name())
	}
	if got := parserkit.ReadsAgentsOf(subject); !slices.Equal(got, []string{"codex-cli"}) {
		t.Errorf("reads agents %v", got)
	}
	if got := parserkit.MarksOf(subject); got != (parserkit.Marks{Sync: true, Origin: true}) {
		t.Errorf("marks %+v", got)
	}
	if parserkit.HasSecondary(subject) {
		t.Error("the parser has no secondary source")
	}
}
