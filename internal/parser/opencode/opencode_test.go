package opencode

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The rows each file gives are proved against the Python analyzer by the
// parser replay (internal/parity/replay); these tests cover the helpers,
// with the oracle's results.

// decoded returns the JSON text decoded, failing the test when it is not
// JSON.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestToolSummaryNamesTheCallsAction(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"a command wins over its description", `{"command": "ls", "description": "list"}`, "ls"},
		{"a file path", `{"filePath": "/a"}`, "/a"},
		{"a search joins pattern and path", `{"pattern": "foo", "path": "/src"}`, "foo | /src"},
		{"an empty path is not joined", `{"pattern": "foo", "path": ""}`, "foo"},
		{"an unknown input is sorted JSON", `{"z": 1, "a": 2}`, `{"z":1,"a":2}`},
		{"null is empty", `null`, ""},
		{"an empty string is empty", `""`, ""},
		{"an empty object is empty", `{}`, ""},
		{"an empty array is JSON", `[]`, "[]"},
		{"a string is itself", `"raw"`, "raw"},
		{"an empty field is not an argument", `{"command": ""}`, `{"command":""}`},
		{"a number field is skipped", `{"command": 5, "query": "q"}`, "q"},
		{"zero is JSON", `0`, "0"},
		{"non-ASCII text is kept", `{"é": "ü"}`, `{"é":"ü"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ToolSummary(decoded(t, test.input)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDecisionTextNamesARefusal(t *testing.T) {
	tests := []struct {
		name, failure, want string
	}{
		{"a 2.0 declined message", `"The user declined this tool call"`, "permission decision: rejected"},
		{"an aborted decline", `{"type":"aborted","message":"The user declined this tool call"}`,
			"permission decision: rejected"},
		{"a 2.0 rejection", `{"type":"permission.rejected"}`,
			"permission decision: rejected (permission.rejected)"},
		{"a 1.x rejection with feedback",
			`"The user rejected permission to use this specific tool call with the following feedback: no"`,
			"permission decision: rejected with feedback"},
		{"a 1.x rejection", `"The user rejected permission to use this specific tool call."`,
			"permission decision: rejected"},
		{"a 1.x deny rule",
			`"The user has specified a rule which prevents you from using this specific tool call. x"`,
			"permission decision: denied by rule"},
		{"a decline of another type", `{"type":"other","message":"The user declined this tool call"}`, ""},
		{"a number", `5`, ""},
		{"another error", `"boom"`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := decisionText(decoded(t, test.failure)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestErrorTextJoinsNameAndMessage(t *testing.T) {
	tests := []struct {
		name, failure, want string
	}{
		{"a string", `"plain"`, "plain"},
		{"a name and a data message", `{"name":"APIError","data":{"message":"rate"}}`, "APIError rate"},
		{"a type and a message", `{"type":"t","message":"m"}`, "t m"},
		{"neither is JSON", `{"x":1}`, `{"x":1}`},
		{"an empty object", `{}`, ""},
		{"null", `null`, ""},
		{"a number", `5`, "5"},
		{"an empty array", `[]`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := errorText(decoded(t, test.failure)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSessionRulesReadBothShapes(t *testing.T) {
	tests := []struct {
		name, text string
		want       []rule
		ok         bool
	}{
		{"a 2.0 rule", `[{"action":"read","resource":"*","effect":"allow"}]`,
			[]rule{{"read", "*", "allow"}}, true},
		{"1.x rules, missing fields and other entries",
			`[{"permission":"bash","pattern":"","action":null}, 3, {"x":1}, ` +
				`{"permission":0,"pattern":false,"action":[]}]`,
			[]rule{{"bash", "-", "-"}, {"0", "false", "[]"}}, true},
		{"an object is not an array", `{}`, nil, false},
		{"text that is not JSON", `x`, nil, false},
		{"an empty array", `[]`, nil, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := sessionRules(test.text)
			if ok != test.ok || !slices.Equal(got, test.want) {
				t.Errorf("got %v, %v; want %v, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestWantsAndSecondaryFollowTheBuildsPaths(t *testing.T) {
	tests := []struct {
		rel              string
		wants, secondary bool
	}{
		{".local/share/opencode/opencode.db", true, false},
		{".local/share/opencode/opencode-beta.db", true, false},
		{".local/share/opencode/opencode-next.db", true, true},
		{".local/share/opencode/opencode-a/b.db", false, false},
		{".local/share/opencode/kilo.db", false, false},
		{".local/share/opencode/storage/session/proj/ses_1.json", true, false},
		{".local/share/opencode/storage/message/ses_1/msg_1.json", true, false},
		{".local/share/opencode/storage/part/msg_1/prt_1.json", false, false},
		{".local/share/opencode/project/p/storage/session/info/ses.json", true, false},
		{".local/share/opencode/project/p/storage/session/message/ses/msg.json", true, false},
		{".local/state/opencode/prompt-history.jsonl", true, true},
		{".local/state/opencode/prompt-stash.jsonl", true, false},
		{".local/state/opencode/session.json", true, true},
		{".local/state/opencode/kv.json", false, false},
		{".local/share/kilo/kilo.db", false, false},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			if got := subject.Wants(artifact); got != test.wants {
				t.Errorf("wants %v, want %v", got, test.wants)
			}
			if got := parser.IsSecondary(subject, artifact); got != test.secondary {
				t.Errorf("secondary %v, want %v", got, test.secondary)
			}
		})
	}
	if subject.Agent() != "opencode" || parser.MarksOf(subject) != (parser.Marks{Sync: true}) {
		t.Errorf("agent %q, marks %+v", subject.Agent(), parser.MarksOf(subject))
	}
}

func TestANameFromTheEvidenceNeverLeavesTheTree(t *testing.T) {
	root := t.TempDir()
	for _, names := range [][]string{{"part", ".."}, {"project", "../x.json"}, {"a\\b"}, {""},
		{"."}, {"x\x00y"}} {
		if _, ok := childPath(root, names...); ok {
			t.Errorf("%q was accepted", names)
		}
	}
	if got, ok := childPath(root, "part", "msg_1"); !ok || got != filepath.Join(root, "part", "msg_1") {
		t.Errorf("got %q, %v", got, ok)
	}
}

func TestChildrenListsRegularJSONFilesInNameOrder(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"b.json", "a.json", ".json", "..json", "c.txt", "x..json"} {
		if err := osfile.WriteFile(filepath.Join(directory, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := osfile.MkdirAll(filepath.Join(directory, "d.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	want := []string{"a.json", "b.json", "x..json"}
	if got := children(directory); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
