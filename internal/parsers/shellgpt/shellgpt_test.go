package shellgpt

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// The rows of the chat, settings and cache files are checked by the golden
// output of the analysis commands (internal/analyze), over inputs that hold
// every parsed agent; these tests cover the helpers the rows are built from.

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
			var problems parserkit.Problems
			messages, failure := loadMessages(nil, test.raw, &problems)
			text := ""
			if failure != nil {
				text = failure.Error()
			}
			if len(messages) != test.messages || (failure != nil) != (test.failure != "") ||
				(test.failure != "?" && !strings.HasPrefix(text, test.failure)) {
				t.Errorf("got %d messages and %q, want %d and %q", len(messages), text,
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
		{"an object without one is compact JSON in its own key order", `{"b":1,"a":[1,2.5]}`,
			`{"b":1,"a":[1,2.5]}`},
		{"null", `null`, ""},
		{"a number", `5`, "5"},
		{"a boolean", `true`, "true"},
		{"an array is JSON", `["x",1]`, `["x",1]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arguments, err := jsonvalue.DecodeString(test.arguments)
			if err != nil {
				t.Fatal(err)
			}
			if got := callText(nil, arguments); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestReadSettingsKeepsOnlyPermissionAndModelKeys(t *testing.T) {
	raw := "A=1\nDEFAULT_MODEL=gpt\r\nbad line\n# c\n  OPENAI_USE_FUNCTIONS = x \n\nSHELL_INTERACTION=true=1"
	var problems parserkit.Problems
	got := readSettings(&input.Artifact{}, raw, &problems)
	want := []setting{{"DEFAULT_MODEL", "gpt"}, {"SHELL_INTERACTION", "true=1"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(problems) != 1 || problems[0].Line != 3 || problems[0].Raw != "" ||
		problems[0].Message != `unparseable line 3 (no "=" separator): bad line` {
		t.Errorf("got problems %+v", problems)
	}
}

func TestAFunctionIsReadWhateverTheASCIICaseOfItsFixedPathParts(t *testing.T) {
	rows := parseSecrets(t, secrets.Setting{}, ".Config/Shell_GPT/FUNCTIONS/Run.PY", "")
	want := "permission setting: function Run.PY (from .Config/Shell_GPT/FUNCTIONS, file mtime)"
	if len(rows) != 1 || rows[0].Text != want {
		t.Errorf("rows %+v, want one %q", rows, want)
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
		{"shell_gpt/cache/0123456789ABCDEF0123456789abcdef", true, true},
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
