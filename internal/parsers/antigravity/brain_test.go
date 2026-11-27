package antigravity

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

const brainRel = ".gemini/antigravity-cli/brain/conv1/.system_generated/logs/transcript_full.jsonl"

func TestARowSaysItsFieldWasTruncatedOnlyWhenTheLineListsIt(t *testing.T) {
	tests := []struct {
		name, line string
		want       []string
	}{
		{"a listed field is marked once though listed twice",
			`{"type": "PLANNER_RESPONSE", "content": "hi", "truncated_fields": ["content", "content"]}`,
			[]string{"hi [truncated: content]"}},
		{"a field not listed is not marked",
			`{"type": "PLANNER_RESPONSE", "content": "hi", "truncated_fields": ["tool_calls"]}`,
			[]string{"hi"}},
		{"every tool call of a line listing tool_calls is marked, whatever the order of the list",
			`{"type": "PLANNER_RESPONSE", "content": "hi", "truncated_fields": ["tool_calls", 5, "x"],` +
				` "tool_calls": [{"name": "a"}, {"name": "b"}]}`,
			[]string{"hi", "{} [truncated: tool_calls]", "{} [truncated: tool_calls]"}},
		{"a later line does not keep an earlier line's list",
			`{"type": "USER_INPUT", "content": "one", "truncated_fields": ["content"]}` + "\n" +
				`{"type": "USER_INPUT", "content": "two"}`,
			[]string{"one [truncated: content]", "two"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseFile(t, brainRel, []byte(test.line+"\n")); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestAPathSpelledInAnotherCaseGivesTheSameRows(t *testing.T) {
	rowsAt := func(rel string) []string {
		home := t.TempDir()
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(`{"type": "USER_INPUT", "content": "hi"}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var rows []string
		artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
		if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
			rows = append(rows, row.SessionID+" "+row.Text)
		}); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	usual := rowsAt(".gemini/antigravity-cli/brain/Conv1/.system_generated/logs/transcript_full.jsonl")
	got := rowsAt(".GEMINI/Antigravity-CLI/BRAIN/Conv1/.System_Generated/LOGS/Transcript_Full.JSONL")
	if !slices.Equal(usual, []string{"Conv1 hi"}) || !slices.Equal(got, usual) {
		t.Errorf("rows %q, want %q, as %q", got, usual, []string{"Conv1 hi"})
	}
}

// A line gives a row per tool call, and each row asks whether its field
// was truncated, so the line's truncated_fields is read once for the line
// rather than once per row.
func TestALinesTruncatedFieldsAreReadOnceForAllItsRows(t *testing.T) {
	for _, calls := range []int{50, 400} {
		t.Run(strconv.Itoa(calls)+" calls and as many truncated names read each name once", func(t *testing.T) {
			names := slices.Repeat([]string{`"x"`}, calls)
			line := `{"type": "PLANNER_RESPONSE", "tool_calls": [` + strings.Repeat(`{},`, calls-1) + `{}],` +
				` "truncated_fields": [` + strings.Join(names, ",") + `]}` + "\n"
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(brainRel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(line), 0o600); err != nil {
				t.Fatal(err)
			}
			subject := New()
			subject.Work = parserkit.CountingWork()
			rows := 0
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + brainRel, Rel: brainRel,
				Home: &input.Home{DiskPath: home}}
			if err := subject.Parse(artifact, parserkit.Options{}, func(timeline.Row) { rows++ }); err != nil {
				t.Fatal(err)
			}
			if reads := subject.Work.Count("truncated field reads"); rows != calls || reads != calls {
				t.Errorf("%d rows read %d truncated names, want %d rows reading %d", rows, reads, calls, calls)
			}
		})
	}
}
