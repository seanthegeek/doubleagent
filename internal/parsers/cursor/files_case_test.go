package cursor

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// transcriptRows returns the rows of an agent transcript named name in a
// home whose state.vscdb holds a composer of each id, in the project
// /srv/<id>, and the transcript's artifact.
func transcriptRows(t *testing.T, ids []string, name string) ([]timeline.Row, *input.Artifact) {
	t.Helper()
	var keyValues [][]any
	for _, ident := range ids {
		keyValues = append(keyValues, []any{"composerData:" + ident, `{"composerId":"` + ident +
			`","workspaceIdentifier":{"uri":{"fsPath":"/srv/` + ident + `"}}}`})
	}
	state, err := sqlitetest.Build(
		sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)"},
		sqlitetest.Table{Name: "cursorDiskKV", SQL: "CREATE TABLE cursorDiskKV (key TEXT, value BLOB)", Rows: keyValues})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	rel := ".cursor/projects/p/agent-transcripts/" + name + "/" + name + ".jsonl"
	for file, data := range map[string][]byte{stateRel: state,
		rel: []byte(`{"role":"user","message":{"content":"<user_query>hi</user_query>"}}` + "\n")} {
		path := filepath.Join(home, filepath.FromSlash(file))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/home/u/" + rel,
		Rel: rel, Agent: "cursor", Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}}
	var rows []timeline.Row
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows, artifact
}

// A transcript's file name is looked up among the composer ids ignoring
// ASCII case: its rows take the store's spelling and project, and keep the
// disk's spelling in source_file. Of two ids equal under the fold the
// name's own spelling wins, and a name that is neither takes none.
func TestATranscriptNamedInAnotherCaseTakesTheComposerIDTheStoreSpells(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, file, session, project string
		ids                          []string
	}{
		{"the store's own spelling", "abc", "abc", "/srv/abc", []string{"abc"}},
		{"another case takes the store's spelling", "ABC", "abc", "/srv/abc", []string{"abc"}},
		{"of two ids the exact spelling wins", "ABC", "ABC", "/srv/ABC", []string{"abc", "ABC"}},
		{"a name that is neither of two ids takes none", "Abc", "Abc", "", []string{"abc", "ABC"}},
		{"a name no id matches is kept", "abc", "abc", "", []string{"xyz"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rows, artifact := transcriptRows(t, test.ids, test.file)
			if len(rows) != 1 {
				t.Fatalf("rows %+v, want one", rows)
			}
			row := rows[0]
			if row.SessionID != test.session || row.ProjectPath != test.project || row.SourceFile != artifact.Original {
				t.Errorf("session %q project %q source %q, want %q, %q and %q", row.SessionID, row.ProjectPath,
					row.SourceFile, test.session, test.project, artifact.Original)
			}
		})
	}
}
