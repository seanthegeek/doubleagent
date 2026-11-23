package kiro

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// conversationProblems returns the texts of the problem rows the parser
// gives a conversation document.
func conversationProblems(document string) []string {
	var found []string
	New().conversation(&input.Artifact{}, parserkit.Options{}, document, "", 3, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, "parser: ") {
			found = append(found, row.Text)
		}
	})
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

func TestAConversationMemberThatIsNotTheRightKindIsReported(t *testing.T) {
	const prompt = `"user": {"content": {"Prompt": {"prompt": "hi"}}}`
	tests := []struct {
		name, member, document string
		array                  bool
	}{
		{"history", "history", `{"conversation_id": "c", "history": %s}`, true},
		{"checkpoints", "checkpoint_manager.checkpoints", `{"history": [], "checkpoint_manager": {"checkpoints": %s}}`, true},
		{"history_snapshot", "checkpoint_manager.checkpoints[].history_snapshot",
			`{"history": [], "checkpoint_manager": {"checkpoints": [{"tag": "1", "history_snapshot": %s}]}}`, true},
		{"main_history", "tangent_state.main_history", `{"history": [], "tangent_state": {"main_history": %s}}`, true},
		{"message_meta_tags", "request_metadata.message_meta_tags",
			`{"history": [{` + prompt + `, "request_metadata": {"message_meta_tags": %s}}]}`, true},
		{"tool_uses", "tool_uses",
			`{"history": [{` + prompt + `, "assistant": {"Response": {"content": "x", "tool_uses": %s}}}]}`, true},
		{"tool_use_results", "tool_use_results",
			`{"history": [{"user": {"content": {"ToolUseResults": {"tool_use_results": %s}}}}]}`, true},
		{"checkpoint_manager", "checkpoint_manager", `{"history": [], "checkpoint_manager": %s}`, false},
		{"tangent_state", "tangent_state", `{"history": [], "tangent_state": %s}`, false},
		{"request_metadata", "request_metadata", `{"history": [{` + prompt + `, "request_metadata": %s}]}`, false},
		{"user", "user", `{"history": [{"user": %s}]}`, false},
		{"user content", "user.content", `{"history": [{"user": {"content": %s}}]}`, false},
		{"assistant", "assistant", `{"history": [{` + prompt + `, "assistant": %s}]}`, false},
		{"an assistant variant's body", "assistant.Response", `{"history": [{` + prompt + `, "assistant": {"Response": %s}}]}`,
			false},
		{"a user content variant's body", "user.content.Prompt", `{"history": [{"user": {"content": {"Prompt": %s}}}]}`,
			false},
	}
	for _, test := range tests {
		kind, silent := "object", []string{`null`, `{}`}
		if test.array {
			kind, silent = "array", []string{`null`, `[]`}
		}
		for _, wrong := range wrongKinds(test.array) {
			t.Run(test.name+" as "+wrong[0], func(t *testing.T) {
				got := conversationProblems(fmt.Sprintf(test.document, wrong[1]))
				if want := []string{"parser: " + test.member + " is not a JSON " + kind}; !slices.Equal(got, want) {
					t.Errorf("problems %q, want %q", got, want)
				}
			})
		}
		for _, value := range silent {
			t.Run(test.name+" as "+value, func(t *testing.T) {
				if got := conversationProblems(fmt.Sprintf(test.document, value)); len(got) != 0 {
					t.Errorf("problems %q, want none", got)
				}
			})
		}
	}
	t.Run("absent members", func(t *testing.T) {
		if got := conversationProblems(`{"history": [{` + prompt + `}]}`); len(got) != 0 {
			t.Errorf("problems %q, want none", got)
		}
	})
}

func TestConversationEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	const prompt = `"user": {"content": {"Prompt": {"prompt": "hi"}}}`
	tests := []struct {
		name, member, document string
	}{
		{"history", "history", `{"conversation_id": "c", "history": %s}`},
		{"checkpoints", "checkpoint_manager.checkpoints", `{"history": [], "checkpoint_manager": {"checkpoints": %s}}`},
		{"history_snapshot", "checkpoint_manager.checkpoints[].history_snapshot",
			`{"history": [], "checkpoint_manager": {"checkpoints": [{"tag": "1", "history_snapshot": %s}]}}`},
		{"main_history", "tangent_state.main_history", `{"history": [], "tangent_state": {"main_history": %s}}`},
		{"tool_uses", "tool_uses",
			`{"history": [{` + prompt + `, "assistant": {"Response": {"content": "x", "tool_uses": %s}}}]}`},
		{"tool_use_results", "tool_use_results",
			`{"history": [{"user": {"content": {"ToolUseResults": {"tool_use_results": %s}}}}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := conversationProblems(fmt.Sprintf(test.document, `[7, "x"]`))
			if want := []string{"parser: " + test.member + " holds 2 entries that are not JSON objects"}; !slices.Equal(got, want) {
				t.Errorf("problems %q, want %q", got, want)
			}
		})
	}
}

// repeatedEntries returns a history of n copies of entry, joined as a JSON
// array.
func repeatedEntries(entry string, n int) string {
	return "[" + strings.TrimSuffix(strings.Repeat(entry+",", n), ",") + "]"
}

func TestAMemberOfTheWrongKindInManyEntriesIsOneProblem(t *testing.T) {
	const prompt = `"user": {"content": {"Prompt": {"prompt": "hi"}}}`
	tests := []struct {
		name, entry, want string
	}{
		{"request_metadata", `{` + prompt + `, "request_metadata": 7}`,
			"request_metadata is not a JSON object in 300 entries"},
		{"user", `{"user": 7}`, "user is not a JSON object in 300 entries"},
		{"assistant", `{` + prompt + `, "assistant": 7}`, "assistant is not a JSON object in 300 entries"},
		{"an assistant variant's body", `{` + prompt + `, "assistant": {"Response": 7}}`,
			"assistant.Response is not a JSON object in 300 entries"},
		{"message_meta_tags", `{` + prompt + `, "request_metadata": {"message_meta_tags": 7}}`,
			"request_metadata.message_meta_tags is not a JSON array in 300 entries"},
		{"tool_uses entries", `{` + prompt + `, "assistant": {"Response": {"content": "x", "tool_uses": [7]}}}`,
			"tool_uses holds 300 entries that are not JSON objects"},
		{"tool_use_results entries", `{"user": {"content": {"ToolUseResults": {"tool_use_results": [7]}}}}`,
			"tool_use_results holds 300 entries that are not JSON objects"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := conversationProblems(`{"history": ` + repeatedEntries(test.entry, 300) + `}`)
			if want := []string{"parser: " + test.want}; !slices.Equal(got, want) {
				t.Errorf("problems %q, want %q", got, want)
			}
		})
	}
}

func TestAHistorySnapshotOfTheWrongKindInManyCheckpointsIsOneProblem(t *testing.T) {
	checkpoints := repeatedEntries(`{"tag": "1", "history_snapshot": 7}`, 300)
	got := conversationProblems(`{"history": [], "checkpoint_manager": {"checkpoints": ` + checkpoints + `}}`)
	want := []string{"parser: checkpoint_manager.checkpoints[].history_snapshot is not a JSON array in 300 entries"}
	if !slices.Equal(got, want) {
		t.Errorf("problems %q, want %q", got, want)
	}
}

func TestEachDifferentVariantTagWithABodyOfTheWrongKindIsItsOwnProblemInOrder(t *testing.T) {
	const n = 3000
	var entries []string
	var want []string
	for i := range n {
		tag := fmt.Sprintf("Tag%d", i)
		entries = append(entries, fmt.Sprintf(`{"user": {"content": {%q: 1}}}`, tag))
		want = append(want, "parser: user.content."+tag+" is not a JSON object")
	}
	got := conversationProblems(`{"history": [` + strings.Join(entries, ",") + `]}`)
	if len(got) != n || got[0] != want[0] || got[n-1] != want[n-1] {
		t.Errorf("%d problems, first %q, want %d with first %q", len(got), got[:min(1, len(got))], n, want[0])
	}
}
