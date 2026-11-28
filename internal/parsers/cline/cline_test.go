package cline

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// The fixed parts of a path match in any ASCII case, as the catalog matches
// them, and the session id keeps the spelling the disk gave.
func TestAManifestPathSpelledInAnotherCaseIsRead(t *testing.T) {
	const manifest = `{"cwd": "/srv/proj", "started_at": 1791021700000, "model": "m"}`
	const respelled = ".CLINE/Data/Sessions/Ab1/Ab1.JSON"
	want := parseSecrets(t, secrets.Setting{}, ".cline/data/sessions/Ab1/Ab1.json", manifest)
	got := parseSecrets(t, secrets.Setting{}, respelled, manifest)
	if !New().Wants(&input.Artifact{Rel: respelled}) || len(want) == 0 || len(got) != len(want) || got[0].SessionID != "Ab1" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range got {
		got[i].SourceFile = want[i].SourceFile
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

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
			if got := denial(nil, content); got != test.want {
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
		problems parserkit.Problems
	}{
		{"cut after a comma", `{"messages": [{"id": "m1"}, {"id": "m2"},`, 2,
			parserkit.Problems{{Line: 3, Message: "array cut off after record 2"}}},
		{"cut after the bracket", `{"messages": [`, 0,
			parserkit.Problems{{Line: 1, Message: "array cut off after record 0"}}},
		{"cut inside a message", `{"messages": [{"id": "m1"}, {"id"`, 1, parserkit.Problems{{Line: 2,
			Message: "unparseable record 2 (invalid JSON at line 1, column 34: unexpected end of JSON input)",
			Raw:     `{"id"`}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "s.messages.json")
			if err := osfile.WriteFile(path, []byte(test.text), 0o600); err != nil {
				t.Fatal(err)
			}
			var problems parserkit.Problems
			result := readTranscript(&input.Artifact{DiskPath: path}, sessionManifest{}, &problems)
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

// A database the parser cannot open is named in its row.
func TestOpenDatabaseNamesTheDatabaseInItsRow(t *testing.T) {
	tests := []struct {
		name, data, prefix string
	}{
		{"a text file", "not sqlite", "sessions.db is not a SQLite database"},
		{"a header over a damaged body", "SQLite format 3\x00" + strings.Repeat("\x00", 200),
			"sessions.db unreadable ("},
		{"no file", "", "sessions.db could not be read ("},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sessions.db")
			if test.data != "" {
				if err := osfile.WriteFile(path, []byte(test.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			database, err := openDatabase(&input.Artifact{}, path, "sessions.db")
			if database != nil {
				database.Close()
			}
			if err == nil || !strings.HasPrefix(err.Error(), test.prefix) {
				t.Errorf("got %v, want a row starting %q", err, test.prefix)
			}
		})
	}
}

func TestAnEnvironmentBlockIsASystemRowInATaskAndASession(t *testing.T) {
	const messages = `[{"role": "user", "content": [{"type": "text", "text": "fix it"},
		{"type": "text", "text": " <environment_details>\ncwd /srv/proj\n</environment_details>"},
		{"type": "text", "text": "see <environment_details> below"}]}]`
	for _, test := range []struct{ name, rel, text string }{
		{"a legacy task", ".config/Code/User/globalStorage/saoudrizwan.claude-dev/tasks/t1/api_conversation_history.json",
			messages},
		{"an SDK session", ".cline/data/sessions/s/s.messages.json", `{"messages": ` + messages + `}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, row := range parseSecrets(t, secrets.Setting{}, test.rel, test.text) {
				got = append(got, row.TurnType+" "+row.Text)
			}
			want := []string{"user fix it", "system <environment_details>\ncwd /srv/proj\n</environment_details>",
				"user see <environment_details> below"}
			if !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
}
