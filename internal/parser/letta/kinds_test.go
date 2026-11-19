package letta

import (
	"path/filepath"
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
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "letta",
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

// checkKind parses a file built around each value of a member and checks
// that the problem row appears once for a string, a number and a value of
// the other container kind, and not for null, an empty value or an absent
// member. The empty value is empty, given by the caller.
func checkKind(t *testing.T, rel, member string, kind jsonvalue.Kind, other, empty string, build func(value string) string) {
	t.Helper()
	problem := parser.ProblemPrefix + parser.WrongKind(member, kind)
	tests := []struct {
		name, json string
		want       int
	}{
		{"a string", `"oops"`, 1}, {"a number", `7`, 1}, {"the other kind", other, 1},
		{"null", `null`, 0}, {"empty", empty, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := countOf(parseFile(t, rel, build(test.json)), problem); got != test.want {
				t.Errorf("%d rows %q, want %d", got, problem, test.want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := countOf(parseFile(t, rel, build("")), problem); got != 0 {
			t.Errorf("%d rows %q, want none", got, problem)
		}
	})
}

// member returns the JSON member `"name": value`, or nothing for no value,
// with a trailing comma.
func member(name, value string) string {
	if value == "" {
		return ""
	}
	return `"` + name + `": ` + value + `, `
}

func TestASettingsMemberThatIsNotAnObjectIsReported(t *testing.T) {
	for _, name := range []string{"sessionsByServer", "permissions", "env"} {
		t.Run(name, func(t *testing.T) {
			checkKind(t, ".letta/settings.json", name, jsonvalue.Object, `[1]`, `{}`,
				func(value string) string { return `{` + member(name, value) + `"x": 1}` })
		})
	}
}

func TestACronsTasksThatIsNotAnArrayIsReported(t *testing.T) {
	checkKind(t, ".letta/crons.json", "tasks", jsonvalue.Array, `{"id": "a"}`, `[]`,
		func(value string) string { return `{` + member("tasks", value) + `"x": 1}` })
}

func TestAnInterruptedTurnsResultsThatIsNotAnArrayIsReported(t *testing.T) {
	checkKind(t, ".letta/listener-state/0123456789abcdef01234567/a_c.json", "results", jsonvalue.Array,
		`{"tool_call_id": "a"}`, `[]`,
		func(value string) string { return `{` + member("results", value) + `"agentId": "a"}` })
}

func TestEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	tests := []struct {
		name, rel, text, want string
	}{
		{"routes", ".letta/channels/telegram/routing.json", `{"routes": [{"chatId": "1"}, 7]}`,
			"parser: routes holds 1 entry that is not a JSON object"},
		{"tasks", ".letta/crons.json", `{"tasks": [{"id": "a"}, "x", 7]}`,
			"parser: tasks holds 2 entries that are not JSON objects"},
		{"results", ".letta/listener-state/0123456789abcdef01234567/a_c.json",
			`{"agentId": "a", "results": [{"tool_call_id": "a"}, 7]}`,
			"parser: results holds 1 entry that is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := countOf(parseFile(t, test.rel, test.text), test.want); got != 1 {
				t.Errorf("%q given %d times, want once", test.want, got)
			}
		})
	}
}
