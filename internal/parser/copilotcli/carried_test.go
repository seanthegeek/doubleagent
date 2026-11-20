package copilotcli

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A working directory and a model of 100,000 bytes each are carried onto
// every row after them, the project path and the model: each row holds them
// cut, so the output grows by 8,329 bytes per message, not by their
// length, and the kit adds one problem row per field.
func TestALongWorkingDirectoryAndModelAreCutInEveryRow(t *testing.T) {
	cwd := "/w/" + strings.Repeat("p", 100_000)
	model := strings.Repeat("m", 100_000)
	tests := []struct {
		messages, size int
	}{
		{500, 4_385_625},
		{1000, 8_550_127},
	}
	for _, test := range tests {
		t.Run(strconv.Itoa(test.messages)+" messages", func(t *testing.T) {
			lines := []string{`{"type":"session.start","timestamp":"2026-01-02T03:04:05Z","data":{"context":{"cwd":"` +
				cwd + `"}}}`, `{"type":"session.model_change","data":{"newModel":"` + model + `"}}`}
			for i := range test.messages {
				lines = append(lines, `{"type":"user.message","data":{"content":"m`+strconv.Itoa(i)+`"}}`)
			}
			rel := sessionDir + "events.jsonl"
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "copilot-cli",
				Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
			var rows []timeline.Row
			size := 0
			if err := parsertest.ParseWithoutSecrets(t, New(), artifact, parser.Options{}, func(row timeline.Row) {
				rows = append(rows, row)
				for _, field := range row.Strings() {
					size += len(field)
				}
			}); err != nil {
				t.Fatal(err)
			}
			// The session start and the model rows precede the messages.
			want := []string{parser.ProblemPrefix + parser.CarriedProblem("project_path", test.messages+2),
				parser.ProblemPrefix + parser.CarriedProblem("model", test.messages+1)}
			if size != test.size || rows[len(rows)-2].Text != want[0] || rows[len(rows)-1].Text != want[1] {
				t.Errorf("%d rows of %d bytes ending %q and %q, want %d bytes, %q", len(rows), size,
					rows[len(rows)-2].Text, rows[len(rows)-1].Text, test.size, want)
			}
		})
	}
}
