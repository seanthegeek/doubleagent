package shellgpt

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// The rows of the chat, settings and cache files are proved against the
// Python analyzer by the parser replay (internal/parity/replay); these
// tests port the helpers the Python tests call directly. Expected values
// come from running the analyzer's functions.

func TestLoadMessagesKeepsWhatSurvivesATruncation(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		messages int
		failure  string
	}{
		{"text that is not JSON", "not json", 0, "?"},
		{"an object is not an array", `{"a":1}`, 0, "not a JSON array"},
		{"elements that are not objects are dropped", `[{"role":"user"},5,{"role":"x"}]`, 2, ""},
		{"a cut after two messages", `[{"role":"user"},{"role":"a","content":"x"}, {"bad"`, 2,
			"truncated after 2 message(s): "},
		{"leading space and a dangling comma", `  [{"role":"u"}, 3,`, 1,
			"truncated after 1 message(s): "},
		{"an empty array", `[]`, 0, ""},
		{"empty text", ``, 0, "?"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			messages, failure := loadMessages(test.raw)
			wantsFailure := test.failure != ""
			if len(messages) != test.messages || (failure != "") != wantsFailure ||
				(test.failure != "?" && !strings.HasPrefix(failure, test.failure)) {
				t.Errorf("got %d messages and %q, want %d and %q", len(messages), failure,
					test.messages, test.failure)
			}
		})
	}
}

func TestCallTextPrefersTheShellCommand(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		want      string
	}{
		{"a JSON string with a command", `"{\"shell_command\":\"ls\"}"`, "ls"},
		{"a JSON string without one", `"{\"x\":1}"`, `{"x":1}`},
		{"a string that is not JSON", `"plain"`, "plain"},
		{"a string holding an array", `"[1]"`, "[1]"},
		{"an object with a command", `{"shell_command":"a"}`, "a"},
		{"an object without one is spaced and sorted", `{"b":1,"a":[1,2.5]}`,
			`{"a": [1, 2.5], "b": 1}`},
		{"null", `null`, ""},
		{"a number", `5`, "5"},
		{"a boolean", `true`, "True"},
		{"an array is a Python list", `["x",1]`, "['x', 1]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arguments, err := pyjson.DecodeString(test.arguments)
			if err != nil {
				t.Fatal(err)
			}
			if got := callText(arguments); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestReadSettingsKeepsOnlyPermissionAndModelKeys(t *testing.T) {
	raw := "A=1\nDEFAULT_MODEL=gpt\r\nbad line\n# c\n  OPENAI_USE_FUNCTIONS = x \n\nSHELL_INTERACTION=true=1"
	var problems parser.Problems
	got := readSettings(raw, &problems)
	want := []setting{{"DEFAULT_MODEL", "gpt"}, {"SHELL_INTERACTION", "true=1"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(problems) != 1 || problems[0].Line != 3 || problems[0].Raw != "" ||
		problems[0].Message != `unparseable line 3 (no "=" separator; not shown, the file holds the API key)` {
		t.Errorf("got problems %+v", problems)
	}
}

func TestWantsTheFourFileShapesAtAnyDepth(t *testing.T) {
	tests := []struct {
		rel       string
		wants     bool
		secondary bool
	}{
		{"chat_cache/abc", true, false},
		{"tmp/chat_cache/abc", true, false},
		{"chat_cache/dir/abc", false, false},
		{".config/shell_gpt/.sgptrc", true, false},
		{".config/shell_gpt/functions/run.py", true, false},
		{".config/shell_gpt/functions/run.txt", false, false},
		{"shell_gpt/cache/0123456789abcdef0123456789abcdef", true, true},
		{"shell_gpt/cache/0123456789ABCDEF0123456789abcdef", false, false},
		{"shell_gpt/cache/0123456789abcdef", false, false},
		{"xchat_cache/abc", false, false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			subject := New()
			if subject.Wants(artifact) != test.wants || subject.Secondary(artifact) != test.secondary {
				t.Errorf("got wants %v secondary %v", subject.Wants(artifact),
					subject.Secondary(artifact))
			}
		})
	}
}
