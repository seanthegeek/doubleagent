package openhands

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// kindsConversation is the directory of the conversation the tests write.
const kindsConversation = ".openhands/conversations/0123456789abcdef0123456789abcdef/"

func TestAConversationWithOnlyADamagedMetaFileIsReported(t *testing.T) {
	const meta = kindsConversation + "meta.json"
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"meta.json cut off, with nothing else, is reported", map[string]string{meta: `{`},
			[]string{"parser: meta.json could not be decoded (invalid JSON at line 1, column 2: unexpected end of JSON input)"}},
		{"meta.json that is not an object, with nothing else, is reported", map[string]string{meta: `[1]`},
			[]string{"parser: meta.json is not a JSON object"}},
		{"a whole meta.json gives no row", map[string]string{meta: `{"id": "x"}`}, nil},
		{"base_state.json reports a damaged meta.json instead",
			map[string]string{meta: `{`, kindsConversation + "base_state.json": `{}`}, nil},
		{"the first event reports a damaged meta.json instead", map[string]string{meta: `{`,
			kindsConversation + "events/event-00000-0123abcd.json": `{"kind": "MessageEvent"}`}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := writeTree(t, t.TempDir(), test.files)
			if got := problemTexts(parseRel(t, home, meta)); !slices.Equal(got, test.want) {
				t.Errorf("problem rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestABaseStateMemberOfTheWrongKindIsReported(t *testing.T) {
	tests := []struct {
		name, state string
		want        []string
	}{
		{"usage_to_metrics as a list", `{"stats": {"usage_to_metrics": [1]}}`,
			[]string{"parser: stats.usage_to_metrics is not a JSON object"}},
		{"token_usages as an object", `{"stats": {"usage_to_metrics": {"agent": {"token_usages": {"model": "m"}}}}}`,
			[]string{"parser: stats.usage_to_metrics.agent.token_usages is not a JSON array"}},
		{"token_usages as a string", `{"stats": {"usage_to_metrics": {"agent": {"token_usages": "m"}}}}`,
			[]string{"parser: stats.usage_to_metrics.agent.token_usages is not a JSON array"}},
		{"blocked_actions as a list", `{"blocked_actions": ["a"]}`,
			[]string{"parser: blocked_actions is not a JSON object"}},
		{"blocked_actions as a number", `{"blocked_actions": 1}`,
			[]string{"parser: blocked_actions is not a JSON object"}},
		{"null and empty members", `{"stats": {"usage_to_metrics": {"agent": {"token_usages": []}}}, "blocked_actions": null}`, nil},
		{"absent members", `{}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := writeTree(t, t.TempDir(), map[string]string{kindsConversation + "base_state.json": test.state})
			if got := problemTexts(parseRel(t, home, kindsConversation+"base_state.json")); !slices.Equal(got, test.want) {
				t.Errorf("problem rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnEventMemberOfTheWrongKindIsReported(t *testing.T) {
	const rel = kindsConversation + "events/event-00000-0123abcd.json"
	tests := []struct {
		name, event string
		want        []string
	}{
		{"tool_calls as an object", `{"kind": "MessageEvent", "llm_message": {"role": "assistant", "content": "x", "tool_calls": {"id": "c"}}}`,
			[]string{"parser: tool_calls is not a JSON array"}},
		{"tool_calls as a string", `{"kind": "MessageEvent", "llm_message": {"role": "assistant", "content": "x", "tool_calls": "c"}}`,
			[]string{"parser: tool_calls is not a JSON array"}},
		{"a message's thinking_blocks as a number", `{"kind": "MessageEvent", "llm_message": {"role": "assistant", "content": "x", "thinking_blocks": 2}}`,
			[]string{"parser: thinking_blocks is not a JSON array"}},
		{"an action's thinking_blocks as an object", `{"kind": "ActionEvent", "tool_name": "t", "thinking_blocks": {"thinking": "x"}}`,
			[]string{"parser: thinking_blocks is not a JSON array"}},
		{"null and empty members", `{"kind": "MessageEvent", "llm_message": {"role": "assistant", "content": "x", "tool_calls": null, "thinking_blocks": []}}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := writeTree(t, t.TempDir(), map[string]string{rel: test.event})
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel, Rel: rel,
				Home: &input.Home{DiskPath: home, Original: "/h"}}
			var rows []timeline.Row
			if err := New().Parse(artifact, parser.Options{IncludeThinking: true}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			if got := problemTexts(rows); !slices.Equal(got, test.want) {
				t.Errorf("problem rows %q, want %q", got, test.want)
			}
		})
	}
}
