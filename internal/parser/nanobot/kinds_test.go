package nanobot

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// problemRows returns the problem rows the parser gives text stored at rel.
func problemRows(t *testing.T, rel, text string) []string {
	t.Helper()
	var found []string
	for _, row := range parseFile(t, rel, text) {
		if strings.HasPrefix(row, "parser: ") {
			found = append(found, row)
		}
	}
	return found
}

// wrongKinds returns the values a member must not have, each with a name:
// a string, a number and the container of the other kind than the member's.
func wrongKinds(array bool) [][2]string {
	other := `[1]`
	if array {
		other = `{"a": 1}`
	}
	return [][2]string{{"a string", `"oops"`}, {"a number", `7`}, {"the other container", other}}
}

func TestAMemberThatIsNotTheRightKindIsReported(t *testing.T) {
	tests := []struct {
		name, rel, member, document string
		array                       bool
	}{
		{"tool_calls", ".nanobot/sessions/s.jsonl", "tool_calls",
			`{"role": "assistant", "content": "ok", "tool_calls": %s}` + "\n", true},
		{"tool_events", ".nanobot/webui/w.jsonl", "tool_events",
			`{"event": "message", "text": "hi", "tool_events": %s}` + "\n", true},
		{"edits", ".nanobot/webui/w.jsonl", "edits", `{"event": "file_edit", "edits": %s}` + "\n", true},
		{"approved", ".nanobot/pairing.json", "approved", `{"approved": %s}`, false},
		{"an approved list", ".nanobot/pairing.json", "approved.telegram", `{"approved": {"telegram": %s}}`, true},
		{"pending", ".nanobot/pairing.json", "pending", `{"pending": %s}`, false},
	}
	for _, test := range tests {
		kind, silent := "object", []string{`null`, `{}`}
		if test.array {
			kind, silent = "array", []string{`null`, `[]`}
		}
		for _, wrong := range wrongKinds(test.array) {
			t.Run(test.name+" as "+wrong[0], func(t *testing.T) {
				got := problemRows(t, test.rel, fmt.Sprintf(test.document, wrong[1]))
				if want := []string{"parser: " + test.member + " is not a JSON " + kind}; !slices.Equal(got, want) {
					t.Errorf("problems %q, want %q", got, want)
				}
			})
		}
		for _, value := range silent {
			t.Run(test.name+" as "+value, func(t *testing.T) {
				if got := problemRows(t, test.rel, fmt.Sprintf(test.document, value)); len(got) != 0 {
					t.Errorf("problems %q, want none", got)
				}
			})
		}
	}
	t.Run("absent members", func(t *testing.T) {
		for rel, document := range map[string]string{
			".nanobot/sessions/s.jsonl": `{"role": "assistant", "content": "ok"}` + "\n",
			".nanobot/webui/w.jsonl":    `{"event": "message", "text": "hi"}` + "\n",
			".nanobot/pairing.json":     `{}`,
		} {
			if got := problemRows(t, rel, document); len(got) != 0 {
				t.Errorf("%s: problems %q, want none", rel, got)
			}
		}
	})
}

func TestEntriesOfTheWrongKindAreReportedOncePerList(t *testing.T) {
	tests := []struct {
		name, rel, document, want string
	}{
		{"tool_calls", ".nanobot/sessions/s.jsonl",
			`{"role": "assistant", "content": "ok", "tool_calls": [7, "x"]}` + "\n",
			"parser: tool_calls holds 2 entries that are not JSON objects"},
		{"tool_events", ".nanobot/webui/w.jsonl", `{"event": "message", "text": "hi", "tool_events": [7]}` + "\n",
			"parser: tool_events holds 1 entry that is not a JSON object"},
		{"edits", ".nanobot/webui/w.jsonl", `{"event": "file_edit", "edits": ["x"]}` + "\n",
			"parser: edits holds 1 entry that is not a JSON object"},
		{"pending", ".nanobot/pairing.json", `{"pending": {"a": {"channel": "t"}, "b": 7}}`,
			"parser: pending holds 1 entry that is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := problemRows(t, test.rel, test.document); !slices.Equal(got, []string{test.want}) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}
