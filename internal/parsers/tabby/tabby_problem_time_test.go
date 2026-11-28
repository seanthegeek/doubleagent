package tabby

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A problem row of an event carries the event's line, its completion id and
// project, and no time; the event's other rows carry the event's time.
func TestAnEventsProblemRowsHaveNoTime(t *testing.T) {
	const rel = ".tabby/events/2026-01-01.json"
	for _, test := range []struct {
		name, event, text, session, project string
	}{
		{"a view that is not an object", `{"view": 3}`, "parser: view is not a JSON object", "", ""},
		{"choices that is not an array", `{"completion": {"completion_id": "k", "choices": 5, "prompt": "p",` +
			`"segments": {"git_url": "https://x/r.git"}}}`, "parser: choices is not a JSON array", "k", "https://x/r.git"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			data := `{"user": "1", "ts": 1759312800123, "event": ` + test.event + "}\n"
			if err := osfile.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
			var rows []timeline.Row
			if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			found, timed := 0, 0
			for _, row := range rows {
				switch {
				case strings.HasPrefix(row.Text, parserkit.ProblemPrefix):
					found++
					if row.TimestampUTC != "" || row.SessionID != test.session || row.ProjectPath != test.project ||
						row.SourceLine != 1 {
						t.Errorf("problem row %+v", row)
					}
				case row.TimestampUTC != "":
					timed++
				}
			}
			if found != 1 || timed == 0 {
				t.Errorf("%d problem rows and %d timed other rows in %+v", found, timed, rows)
			}
		})
	}
}
