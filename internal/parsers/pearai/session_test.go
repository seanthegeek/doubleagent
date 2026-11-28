package pearai

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

func TestASessionsProblemRowHasNoTime(t *testing.T) {
	entry := `{"sessionId": "s1", "dateCreated": "2026-10-03T10:00:00.000Z", "workspaceDirectory": "/srv/proj"}`
	long := strings.Repeat("i", parserkit.CarriedLimit+1)
	history := [3]string{"", "/srv/proj", "parser: history is not a JSON array"}
	tests := []struct {
		name, index string
		want        [][3]string
	}{
		{"an index entry", "[" + entry + "]", [][3]string{history}},
		{"an index entry its id repeats, with an integration over the limit", `[` + entry +
			`, {"sessionId": "s1", "dateCreated": "2026-10-04T10:00:00.000Z", "integrationType": "` + long + `"}]`,
			[][3]string{{"", "", parserkit.ProblemPrefix + parserkit.CarriedProblem("integration type", 1)}, history}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			for rel, text := range map[string]string{".pearai/sessions/sessions.json": test.index,
				".pearai/sessions/s1.json": `{"sessionId": "s1", "title": "x", "history": 7}`} {
				path := filepath.Join(home, filepath.FromSlash(rel))
				if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			artifact := &input.Artifact{DiskPath: filepath.Join(home, ".pearai", "sessions", "s1.json"),
				Original: "/h/.pearai/sessions/s1.json", Rel: ".pearai/sessions/s1.json", Home: &input.Home{DiskPath: home}}
			var problems [][3]string
			starts := 0
			if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
				switch {
				case strings.HasPrefix(row.Text, parserkit.ProblemPrefix):
					problems = append(problems, [3]string{row.TimestampUTC, row.ProjectPath, row.Text})
				case strings.HasPrefix(row.Text, "session start: ") && row.TimestampUTC != "":
					starts++
				}
			}); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(problems, test.want) || starts != len(test.want) {
				t.Errorf("problem rows\n%q\nwant\n%q\nand %d timed start rows, want %d", problems, test.want, starts,
					len(test.want))
			}
		})
	}
}

// A session file with no sessionId takes its id from its file name and
// finds its index entry under another ASCII case of the name; the rows take
// the index's spelling, and a sessionId in the file is kept as written.
func TestASessionFileNamedInAnotherCaseFindsItsIndexEntry(t *testing.T) {
	const history = `"history":[{"message":{"role":"user","content":"hi"}}]`
	entries := func(ids ...string) string {
		var written []string
		for _, each := range ids {
			written = append(written, `{"sessionId":"`+each+`","dateCreated":"1767000000000","workspaceDirectory":"/srv/proj"}`)
		}
		return "[" + strings.Join(written, ",") + "]"
	}
	tests := []struct {
		name, index, file, session string
		wantID, wantPath           string
	}{
		{"another case takes the index's spelling", entries("ab1"), "AB1.json", `{"title":"T",` + history + `}`,
			"ab1", "/srv/proj"},
		{"the exact spelling wins over a fold", entries("AB1", "ab1"), "AB1.json", `{"title":"T",` + history + `}`,
			"AB1", "/srv/proj"},
		{"two folds with no exact spelling take none", entries("AB1", "ab1"), "Ab1.json",
			`{"title":"T",` + history + `}`, "Ab1", ""},
		{"a sessionId in the file is kept as written", entries("ab1"), "x.json", `{"sessionId":"AB1",` + history + `}`,
			"AB1", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			rel := ".pearai/sessions/" + test.file
			for written, text := range map[string]string{".pearai/sessions/sessions.json": test.index,
				rel: test.session} {
				path := filepath.Join(home, filepath.FromSlash(written))
				if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
				Rel: rel, Home: &input.Home{DiskPath: home}}
			var rows []timeline.Row
			if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				t.Fatal("no rows")
			}
			for _, row := range rows {
				if row.SessionID != test.wantID || row.ProjectPath != test.wantPath {
					t.Errorf("row %q has session %q, project %q; want %q, %q", row.Text, row.SessionID,
						row.ProjectPath, test.wantID, test.wantPath)
				}
			}
		})
	}
}
