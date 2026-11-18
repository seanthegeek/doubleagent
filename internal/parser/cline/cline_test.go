package cline

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// A manifest is a file named after its own session directory; the names
// are the fixture's.
func TestIsManifestMatchesASessionDirectorysOwnFile(t *testing.T) {
	for _, test := range []struct {
		name, rel string
		want      bool
	}{
		{"the manifest of a session", ".cline/data/sessions/1791021700000_k3x9q/1791021700000_k3x9q.json", true},
		{"its messages file", ".cline/data/sessions/1791021700000_k3x9q/1791021700000_k3x9q.messages.json", false},
		{"a file named after another session", ".cline/data/sessions/1791021700000_k3x9q/1791000000000_old01.json", false},
		{"a manifest one level deeper", ".cline/data/sessions/a/a/a.json", false},
		{"an empty directory name", ".cline/data/sessions//.json", false},
		{"another root", ".cline/sessions/a/a.json", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isManifest(test.rel); got != test.want {
				t.Errorf("isManifest = %v, want %v", got, test.want)
			}
		})
	}
}

// The expected texts are the permission decisions research/cline.md
// describes. A reason must end the text: one followed by a line break is
// not read, and no runtime writes one.
func TestDenialReadsRejectionsAndBlocks(t *testing.T) {
	for _, test := range []struct {
		name, content, want string
	}{
		{"a rejection as a JSON string",
			`"{\"error\": \"no -- NOT a tool or system failure. Clarify with user before proceeding.\"}"`,
			"permission decision: rejected reason=no"},
		{"a tool disabled by policy", `{"error": "Tool \"x\" is disabled by policy"}`,
			`permission decision: blocked reason=Tool "x" is disabled by policy`},
		{"a call blocked by a hook", `{"error": "Tool a b was blocked by a runtime hook"}`,
			"permission decision: blocked reason=Tool a b was blocked by a runtime hook"},
		{"a block with a trailing line break", `{"error": "Tool a was blocked by a runtime hook\n"}`, ""},
		{"a reason that is not text", `{"error": 5}`, ""},
		{"a string that is not JSON", `"not json"`, ""},
		{"a list", `["x"]`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			content, err := jsonvalue.DecodeString(test.content)
			if err != nil {
				t.Fatal(err)
			}
			if got := denial(content); got != test.want {
				t.Errorf("denial = %q, want %q", got, test.want)
			}
		})
	}
}
