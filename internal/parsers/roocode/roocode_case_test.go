package roocode

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestATaskDirectorySpelledInAnotherCaseTakesItsIndexEntry(t *testing.T) {
	tests := []struct {
		name, ids, session, time, project string
	}{
		{"one id equal ignoring case", `"ab1"`, "ab1", "2025-12-29T09:20:00.000Z", "/srv/proj"},
		{"two ids equal ignoring case", `"ab1"},{"id":"Ab1"`, "AB1", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			const root = ".config/Code/User/globalStorage/rooveterinaryinc.roo-cline/tasks/"
			writeFile(t, home, root+"_index.json", `{"entries":[{"id":`+test.ids+`,"ts":1767000000000,"task":"t",`+
				`"workspace":"/srv/proj","mode":"code"}]}`)
			rel := root + "AB1/api_conversation_history.json"
			writeFile(t, home, rel, `[{"role":"user","content":"hi"}]`)
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel, Original: "/h/" + rel,
				Home: &input.Home{DiskPath: home}}
			var rows []timeline.Row
			if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].SessionID != test.session || rows[0].TimestampUTC != test.time ||
				rows[0].ProjectPath != test.project || !strings.HasSuffix(rows[0].SourceFile, "AB1/api_conversation_history.json") {
				t.Errorf("rows %+v, want session %q at %q in %q from %s", rows, test.session, test.time,
					test.project, rel)
			}
		})
	}
}
