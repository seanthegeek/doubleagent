package pi

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseBytes writes data at rel under a new home and returns the texts of
// the rows the parser gives it.
func parseBytes(t *testing.T, rel string, data []byte) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "pi",
		Home: &input.Home{DiskPath: home}, Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// countOf returns how many of the rows have exactly the given text.
func countOf(rows []string, text string) int {
	total := 0
	for _, row := range rows {
		if row == text {
			total++
		}
	}
	return total
}

func TestABugReportDiagnosticsListThatIsNotAnArrayIsReported(t *testing.T) {
	for _, member := range []string{"crashes", "assistant"} {
		problem := parser.ProblemPrefix + parser.WrongKind("diagnostics."+member, jsonvalue.Array)
		tests := []struct {
			name, json string
			want       int
		}{
			{"a string", `"oops"`, 1}, {"a number", `7`, 1}, {"an object", `{"a": 1}`, 1},
			{"null", `null`, 0}, {"empty", `[]`, 0}, {"absent", "", 0},
		}
		for _, test := range tests {
			t.Run(member+" "+test.name, func(t *testing.T) {
				diagnostics := `{"sessionId": "s"}`
				if test.json != "" {
					diagnostics = `{"sessionId": "s", "` + member + `": ` + test.json + `}`
				}
				data := zipOf(t, [2]string{"report.json", `{"id": "r"}`}, [2]string{"diagnostics.json", diagnostics})
				if got := countOf(parseBytes(t, "pi-bug-report-r.zip", data), problem); got != test.want {
					t.Errorf("%d rows %q, want %d", got, problem, test.want)
				}
			})
		}
	}
}
