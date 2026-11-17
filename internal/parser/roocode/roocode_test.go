package roocode

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// The expected texts come from running api_decisions over a tool_result
// holding the same content.
func TestDecisionTextReadsTheStatusObjectARoundTripWrites(t *testing.T) {
	for _, test := range []struct {
		name, content, want string
		ok                  bool
	}{
		{"a denial", `"{\"status\":\"denied\",\"message\":\"no\"}"`, "permission decision: denied", true},
		{"an approval with feedback and output after it",
			`"  {\"status\":\"approved\",\"feedback\":\"ok\"} then output"`,
			"permission decision: approved feedback=ok", true},
		{"feedback that is an object, in a text block",
			`[{"type": "text", "text": "{\"status\":\"denied\",\"feedback\":{\"a\":true}}"}]`,
			`permission decision: denied feedback={"a":true}`, true},
		{"another status", `"{\"status\":\"other\"}"`, "", false},
		{"an object cut short", `"{\"status\":"`, "", false},
		{"plain text", `"plain"`, "", false},
		{"null feedback is left out", `"{\"status\":\"approved\",\"feedback\":null}"`,
			"permission decision: approved", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			content, err := pyjson.DecodeString(test.content)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := decisionText(content)
			if got != test.want || ok != test.ok {
				t.Errorf("decisionText = %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestTheWorkspaceDatabasePatternFindsAnEditorsWorkspaceState(t *testing.T) {
	for _, test := range []struct {
		name, rel string
		want      bool
	}{
		{"a workspace state.vscdb", ".config/Code/User/workspaceStorage/abc123/state.vscdb", true},
		{"one at the home's top", "User/workspaceStorage/abc123/state.vscdb", true},
		{"the global state", ".config/Code/User/globalStorage/state.vscdb", false},
		{"a backup beside it", ".config/Code/User/workspaceStorage/abc123/state.vscdb.backup", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := workspaceDatabase.MatchString(test.rel); got != test.want {
				t.Errorf("match = %v, want %v", got, test.want)
			}
		})
	}
}
