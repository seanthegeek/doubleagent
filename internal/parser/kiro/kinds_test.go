package kiro

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// conversationProblems returns the texts of the problem rows the parser
// gives a conversation document.
func conversationProblems(document string) []string {
	var found []string
	New().conversation(&input.Artifact{}, parser.Options{}, document, "", 3, func(row timeline.Row) {
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
