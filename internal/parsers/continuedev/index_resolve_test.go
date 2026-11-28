package continuedev

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sessionRows writes index as the sessions.json of a new home and the
// session file at name beside it, and returns the rows the parser gives
// for the session file.
func sessionRows(t *testing.T, index, name, session string) []timeline.Row {
	t.Helper()
	home := t.TempDir()
	write := func(rel, text string) *input.Artifact {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	}
	write(".continue/sessions/"+IndexName, index)
	var rows []timeline.Row
	artifact := write(".continue/sessions/"+name, session)
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// A session file with no sessionId takes its id from its file name and
// finds its index entry under another ASCII case of the name; the rows take
// the index's spelling of the id, and the source file keeps the disk's.
func TestASessionFileNamedInAnotherCaseFindsItsIndexEntry(t *testing.T) {
	const history = `"history":[{"message":{"role":"user","content":"hi"}}]`
	const entry = `{"sessionId":"%s","title":"T","dateCreated":"1767000000000","workspaceDirectory":"/srv/proj"}`
	named := func(ids ...string) string {
		var entries []string
		for _, each := range ids {
			entries = append(entries, strings.Replace(entry, "%s", each, 1))
		}
		return "[" + strings.Join(entries, ",") + "]"
	}
	tests := []struct {
		name, index, file, session string
		wantID, wantTime, wantPath string
	}{
		{"another case takes the index's spelling", named("ab1"), "AB1.json", `{"title":"T",` + history + `}`,
			"ab1", "2025-12-29T09:20:00.000Z", "/srv/proj"},
		{"the exact spelling wins over a fold", named("AB1", "ab1"), "AB1.json", `{"title":"T",` + history + `}`,
			"AB1", "2025-12-29T09:20:00.000Z", "/srv/proj"},
		{"two folds with no exact spelling take none", named("AB1", "ab1"), "Ab1.json", `{"title":"T",` + history + `}`,
			"Ab1", "", ""},
		{"a sessionId in the file is kept as written", named("ab1"), "x.json", `{"sessionId":"AB1",` + history + `}`,
			"AB1", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := sessionRows(t, test.index, test.file, test.session)
			if len(rows) == 0 {
				t.Fatal("no rows")
			}
			for _, row := range rows {
				if row.SessionID != test.wantID || row.TimestampUTC != test.wantTime || row.ProjectPath != test.wantPath {
					t.Errorf("row %q has session %q, time %q, project %q; want %q, %q, %q", row.Text,
						row.SessionID, row.TimestampUTC, row.ProjectPath, test.wantID, test.wantTime, test.wantPath)
				}
				if !strings.HasSuffix(row.SourceFile, test.file) {
					t.Errorf("source file %q does not keep the disk's name %q", row.SourceFile, test.file)
				}
			}
		})
	}
}
