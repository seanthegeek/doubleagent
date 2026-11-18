package cline

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
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

// A messages file cut mid-write keeps the messages read whole and reports
// the cut in the kit's wording.
func TestACutMessagesFileReportsTheCutAfterTheWholeMessages(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		messages int
		problems parser.Problems
	}{
		{"cut after a comma", `{"messages": [{"id": "m1"}, {"id": "m2"},`, 2,
			parser.Problems{{Line: 3, Message: "array cut off after record 2"}}},
		{"cut after the bracket", `{"messages": [`, 0,
			parser.Problems{{Line: 1, Message: "array cut off after record 0"}}},
		{"cut inside a message", `{"messages": [{"id": "m1"}, {"id"`, 1, parser.Problems{{Line: 2,
			Message: "unparseable record 2 (invalid JSON at line 1, column 34: unexpected end of JSON input)",
			Raw:     `{"id"`}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "s.messages.json")
			if err := osfile.WriteFile(path, []byte(test.text), 0o600); err != nil {
				t.Fatal(err)
			}
			var problems parser.Problems
			result := readTranscript(&input.Artifact{DiskPath: path}, jsonvalue.NewNull(), &problems)
			count := 0
			for range result.messages {
				count++
			}
			if count != test.messages || !slices.Equal(problems, test.problems) {
				t.Errorf("got %d messages and %+v, want %d and %+v", count, problems, test.messages,
					test.problems)
			}
		})
	}
}
