package littlecoder

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseFile writes text at rel under a new home and returns the texts of
// the rows the parser gives it.
func parseFile(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "littlecoder",
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

// checkMember parses a file built around each value of a member and checks
// that the problem row appears once for a string, a number and a value of
// the other container kind, and not for null, an empty value or an absent
// member (an empty text for the value).
func checkMember(t *testing.T, rel, member string, kind jsonvalue.Kind, other string, build func(value string) string) {
	t.Helper()
	problem := parser.ProblemPrefix + parser.WrongKind(member, kind)
	empty := `[]`
	if kind == jsonvalue.Object {
		empty = `{}`
	}
	tests := []struct {
		name, json string
		want       int
	}{
		{"a string", `"oops"`, 1}, {"a number", `7`, 1}, {"the other kind", other, 1},
		{"null", `null`, 0}, {"empty", empty, 0}, {"absent", "", 0},
	}
	for _, test := range tests {
		t.Run(member+" "+test.name, func(t *testing.T) {
			if got := countOf(parseFile(t, rel, build(test.json)), problem); got != test.want {
				t.Errorf("%d rows %q, want %d", got, problem, test.want)
			}
		})
	}
}

// field returns `"name": value,` or nothing for no value.
func field(name, value string) string {
	if value == "" {
		return ""
	}
	return `"` + name + `": ` + value + `,`
}

func TestASavedSessionsMessagesThatIsNotAnArrayIsReported(t *testing.T) {
	checkMember(t, ".little-coder/sessions/s.json", "messages", jsonvalue.Array, `{"role": "user"}`,
		func(value string) string { return `{` + field("messages", value) + `"session_id": "s"}` })
	checkMember(t, legacyHistoryRel, "messages", jsonvalue.Array, `{"role": "user"}`, func(value string) string {
		return `{"sessions": [{` + field("messages", value) + `"session_id": "s"}]}`
	})
}

func TestASavedMessagesToolCallsThatIsNotAnArrayIsReported(t *testing.T) {
	message := func(value string) string {
		return `{"role": "assistant", ` + field("tool_calls", value) + `"content": "hi"}`
	}
	checkMember(t, ".little-coder/sessions/s.json", "tool_calls", jsonvalue.Array, `{"id": "c"}`,
		func(value string) string { return `{"session_id": "s", "messages": [` + message(value) + `]}` })
	checkMember(t, legacyHistoryRel, "tool_calls", jsonvalue.Array, `{"id": "c"}`, func(value string) string {
		return `{"sessions": [{"session_id": "s", "messages": [` + message(value) + `]}]}`
	})
}

func TestASnapshotsFileBackupsThatIsNotAnObjectIsReported(t *testing.T) {
	checkMember(t, ".little-coder/checkpoints/s1/snapshots.json", "file_backups", jsonvalue.Object, `[1]`,
		func(value string) string { return `[{` + field("file_backups", value) + `"id": "a"}]` })
}

func TestASnapshotBackupThatIsNotAnObjectIsReportedNotListed(t *testing.T) {
	const rel = ".little-coder/checkpoints/s1/snapshots.json"
	tests := []struct {
		name, backup string
		problem      bool
	}{
		{"an object", `{"backup_filename": "b@v1"}`, false},
		{"an object without a backup", `{"backup_filename": null}`, false},
		{"a string", `"b@v1"`, true},
		{"a number", `1`, true},
		{"null, a file that did not exist", `null`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := parseFile(t, rel, `[{"id": "a", "file_backups": {"/p/a.py": `+test.backup+`}}]`)
			problem := countOf(rows, "parser: file_backups./p/a.py is not a JSON object")
			listed := 0
			for _, row := range rows {
				if strings.Contains(row, "/p/a.py (") {
					listed++
				}
			}
			if (problem == 1) != test.problem || (listed == 1) == test.problem {
				t.Errorf("rows %q", rows)
			}
		})
	}
}
