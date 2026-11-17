package kiro

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The rows of every file are proved against the Python analyzer by the
// parser replay (internal/parity/replay); these tests port what the Python
// tests call directly. Expected values come from running the analyzer.

func TestSalvageKeepsTheEntriesBeforeTheDamage(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		want      string
		wantError bool
	}{
		{"a whole object is the state", `{"a": 1}`, `{"a":1}`, false},
		{"an array is no state", `[1]`, "", true},
		{"text that is not JSON is no state", "garbage", "", true},
		{"a cut entry is dropped and an escaped id decoded",
			`{"conversation_id": "c\"1", "history": [{"x": 1}, {"y": `,
			`{"history":[{"x":1}],"conversation_id":"c\"1"}`, true},
		{"entries that are not objects and stray commas are skipped",
			`{"history": [1, {"a":1} , ,{"b":2}]x`, `{"history":[{"a":1},{"b":2}]}`, true},
		{"an id with a bad escape is left out", `{"conversation_id": "bad\q", "history": [`,
			`{"history":[]}`, true},
		{"an unclosed array keeps its entries", `{"history":[{"a":1}`, `{"history":[{"a":1}]}`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, ok, message := New().salvage(test.text)
			got := ""
			if ok {
				got = state.JSON()
			}
			if got != test.want || (message != "") != test.wantError {
				t.Errorf("got %s, %q; want %s, error %v", got, message, test.want, test.wantError)
			}
		})
	}
}

func TestSalvageTakesJSONWhitespaceOnly(t *testing.T) {
	template := func(first, second, third, fourth string) string {
		return `{"conversation_id"` + first + ":" + second + `"c1", "history"` + third + ":" +
			fourth + `[{"a": 1}, {"b`
	}
	state, _, _ := New().salvage(template(" ", "\n", "\r\n", "\t"))
	if got := state.Get("conversation_id"); !got.IsString() || got.StringValue() != "c1" {
		t.Errorf("got %v", got)
	}
	for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
		state, _, _ := New().salvage(template(space, "", space, ""))
		if state.Has("conversation_id") {
			t.Errorf("%q: the id was read", space)
		}
	}
}

func TestHistoryEscapesAreOnlyLineFeedAndBackslash(t *testing.T) {
	tests := []struct{ line, want string }{
		{`a\nb\\c`, "a\nb\\c"},
		{`C:\tmp\nx`, `C:\tmp\nx`},
		{`trailing\`, `trailing\`},
	}
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			if got := unescapeHistory(test.line); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDecisionTextNamesWhoRefusedTheCall(t *testing.T) {
	rule := " was rejected because the arguments supplied were forbidden"
	tests := []struct {
		prompt jsonvalue.Value
		want   string
	}{
		{jsonvalue.NewNull(), "permission decision: cancelled"},
		{jsonvalue.NewString(""), "permission decision: cancelled"},
		{jsonvalue.NewInt(5), "permission decision: cancelled"},
		{jsonvalue.NewString(denyText), "permission decision: denied by=n"},
		{jsonvalue.NewString(interruptText), "permission decision: cancelled by=interrupt"},
		{jsonvalue.NewString("Tool use with fs_write" + rule), "permission decision: denied by=deny-rule tool=fs_write"},
		{jsonvalue.NewString("Tool use with a\nb" + rule + "\n"), "permission decision: denied by=deny-rule tool=a\nb"},
		{jsonvalue.NewString("Tool use with x" + rule + "\n\n"), "permission decision: denied by=user-text"},
		{jsonvalue.NewString("no thanks"), "permission decision: denied by=user-text"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if got := New().decisionText(test.prompt); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResultTextJoinsTheBlocksAndAddsTheStatus(t *testing.T) {
	tests := []struct{ result, want string }{
		{`{"content": [{"Text": "a"}, {"Json": {"b": 1, "a": 2}}, {"x": 1}, "s", 3], "status": "Success"}`,
			"a\n{\"b\":1,\"a\":2}\n{\"x\":1}\ns\n3 [Success]"},
		{`{"content": [], "status": "Error"}`, "[Error]"},
		{`{"content": "ab"}`, "a\nb"},
		{`{"content": null}`, ""},
		{`{"content": {"k": 1}, "status": 0}`, "k"},
	}
	for _, test := range tests {
		t.Run(test.result, func(t *testing.T) {
			result, err := jsonvalue.DecodeString(test.result)
			if err != nil {
				t.Fatal(err)
			}
			if got := resultText(result); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAJSONFileIsWantedWhenItStartsLikeAnExport(t *testing.T) {
	tests := []struct {
		name    string
		content string
		write   bool
		want    bool
	}{
		{"an export", "  \n{\"conversation_id\": \"c\", \"history\": []}", true, true},
		{"another object", `{"name": "x"}`, true, false},
		{"an array naming the key", `["conversation_id"]`, true, false},
		{"a missing file is offered", "", false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "chat.json")
			if test.write {
				if err := osfile.WriteFile(path, []byte(test.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			artifact := &input.Artifact{DiskPath: path, Rel: "exports/chat.json"}
			if got := New().Wants(artifact); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
