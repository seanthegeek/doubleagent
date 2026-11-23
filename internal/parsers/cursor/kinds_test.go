package cursor

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseText writes text at rel under a new home and returns the texts of
// the rows the parser gives it.
func parseText(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var texts []string
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// decoded returns text decoded as JSON, failing the test when it is not.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatalf("decode %s: %v", text, err)
	}
	return value
}

// messages returns the messages of problems, in order.
func messages(problems parserkit.Problems) []string {
	var found []string
	for _, problem := range problems {
		found = append(found, problem.Message)
	}
	return found
}

func TestATranscriptMessageOrContentOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".cursor/projects/p/agent-transcripts/c/c.jsonl"
	const messageWrong = "parser: message is not a JSON object"
	const contentWrong = "parser: message.content is not a JSON array"
	tests := []struct {
		name, message string
		want          []string
	}{
		{"message as a string", `"hi"`, []string{messageWrong}},
		{"message as a number", `3`, []string{messageWrong}},
		{"message as an array", `[{"content": "hi"}]`, []string{messageWrong}},
		{"message null", `null`, nil},
		{"content as a number", `{"content": 3}`, []string{contentWrong}},
		{"content as an object", `{"content": {"type": "text", "text": "hi"}}`, []string{contentWrong}},
		{"content as text", `{"content": "hi"}`, []string{"hi"}},
		{"content holding a string among blocks", `{"content": [{"type": "text", "text": "hi"}, "x", 2]}`,
			[]string{"hi", "parser: message.content holds 2 entries that are not JSON objects"}},
		{"content null", `{"content": null}`, nil},
		{"content empty", `{"content": []}`, nil},
		{"content absent", `{}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseText(t, rel, `{"role": "assistant", "message": `+test.message+`}`+"\n")
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestACLIConfigPermissionsMemberOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".cursor/cli-config.json"
	const allowed = "permission setting: permissions.allow=Shell(ls) (from .cursor/cli-config.json, file mtime)"
	tests := []struct {
		name, permissions string
		want              []string
	}{
		{"permissions as a string", `"x"`, []string{"parser: permissions is not a JSON object"}},
		{"permissions as a number", `3`, []string{"parser: permissions is not a JSON object"}},
		{"permissions as an array", `["Shell(ls)"]`, []string{"parser: permissions is not a JSON object"}},
		{"permissions null", `null`, nil},
		{"permissions empty", `{}`, nil},
		{"allow as a string", `{"allow": "Shell(ls)"}`, []string{"parser: permissions.allow is not a JSON array"}},
		{"allow as a number", `{"allow": 3}`, []string{"parser: permissions.allow is not a JSON array"}},
		{"deny as an object", `{"allow": ["Shell(ls)"], "deny": {"a": 1}}`,
			[]string{allowed, "parser: permissions.deny is not a JSON array"}},
		{"allow null and deny empty", `{"allow": null, "deny": []}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseText(t, rel, `{"permissions": `+test.permissions+`}`); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("permissions absent", func(t *testing.T) {
		if got := parseText(t, rel, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestAListValueThatIsNotAnArrayIsReported(t *testing.T) {
	const key = "cloudAgentRepository.agents.auth0|user_1"
	const wrong = "cloudAgentRepository.agents.auth0|user_1 is not a JSON array"
	tests := []struct {
		name, value string
		want        []string
		items       int
	}{
		{"a string", `"x"`, []string{wrong}, 0},
		{"a number", `3`, []string{wrong}, 0},
		{"an object", `{"bcId": "bc-1"}`, []string{wrong}, 0},
		{"null", `null`, nil, 0},
		{"empty", `[]`, nil, 0},
		{"an array", `[{"bcId": "bc-1"}]`, nil, 1},
		{"an array holding a string", `[{"bcId": "bc-1"}, "x"]`,
			[]string{"cloudAgentRepository.agents.auth0|user_1 holds 1 entry that is not a JSON object"}, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parserkit.Problems
			items := 0
			for range objectEntries(nil, keyed{rowid: 7, key: key, value: sqlitefile.TextValue(test.value)}, &problems) {
				items++
			}
			if got := messages(problems); !slices.Equal(got, test.want) || items != test.items {
				t.Errorf("problems %q and %d items, want %q and %d", got, items, test.want, test.items)
			}
			for _, problem := range problems {
				if problem.Line != 7 {
					t.Errorf("problem at line %d, want the rowid 7", problem.Line)
				}
			}
		})
	}
}

func TestAComposerDataValueThatIsNotAnObjectIsReported(t *testing.T) {
	const wrong = "composerData:c1 is not a JSON object"
	tests := []struct {
		name, value string
		want        []string
	}{
		{"a string", `"x"`, []string{wrong}},
		{"a number", `3`, []string{wrong}},
		{"an array", `[]`, []string{wrong}},
		{"null", `null`, nil},
		{"empty", `{}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parserkit.Problems
			value := objectRecord(nil, keyed{rowid: 3, key: "composerData:c1", value: sqlitefile.TextValue(test.value)}, &problems)
			if got := messages(problems); !slices.Equal(got, test.want) || !value.IsObject() {
				t.Errorf("problems %q and value %s, want %q and an object", got, value.JSON(), test.want)
			}
		})
	}
}

func TestAnInlineHeaderListThatIsNotAnArrayIsReported(t *testing.T) {
	const wrong = "fullConversationHeadersOnly is not a JSON array"
	tests := []struct {
		name, data string
		want       []string
	}{
		{"a string", `{"fullConversationHeadersOnly": "x"}`, []string{wrong}},
		{"a number", `{"fullConversationHeadersOnly": 3}`, []string{wrong}},
		{"an object", `{"fullConversationHeadersOnly": {"bubbleId": "u1"}}`, []string{wrong}},
		{"null", `{"fullConversationHeadersOnly": null}`, nil},
		{"empty", `{"fullConversationHeadersOnly": []}`, nil},
		{"absent", `{}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := newComposer("c1")
			current.stored = []storedData{{value: decoded(t, test.data), rowid: 5, problems: current.problems}}
			reader := &stateReader{owner: New()}
			reader.entryLists(current)
			if got := messages(*current.problems); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

func TestASettingsBlobMemberOfTheWrongKindIsReported(t *testing.T) {
	tests := []struct {
		name, document string
		want           []string
	}{
		{"composerState as a string", `{"composerState": "x"}`, []string{"composerState is not a JSON object"}},
		{"composerState as an array", `{"composerState": []}`, []string{"composerState is not a JSON object"}},
		{"aiSettings as a number", `{"aiSettings": 3}`, []string{"aiSettings is not a JSON object"}},
		{"modelLastUsedAt as a string", `{"aiSettings": {"modelLastUsedAt": "x"}}`,
			[]string{"aiSettings.modelLastUsedAt is not a JSON object"}},
		{"modelLastUsedAt as an array", `{"aiSettings": {"modelLastUsedAt": ["m"]}}`,
			[]string{"aiSettings.modelLastUsedAt is not a JSON object"}},
		{"a standing list as a string", `{"composerState": {"yoloCommandAllowlist": "ls"}}`,
			[]string{"yoloCommandAllowlist is not a JSON array"}},
		{"a standing list as an object", `{"composerState": {"webFetchDomainAllowlist": {"a": 1}}}`,
			[]string{"webFetchDomainAllowlist is not a JSON array"}},
		{"modes4 as a number", `{"composerState": {"modes4": 3}}`, []string{"modes4 is not a JSON array"}},
		{"modes4 as an object", `{"composerState": {"modes4": {"id": "agent"}}}`, []string{"modes4 is not a JSON array"}},
		{"members null", `{"composerState": {"modes4": null, "yoloCommandDenylist": null}, "aiSettings": null}`, nil},
		{"members empty", `{"composerState": {"modes4": [], "smartAllowlistDenylist": []}, "aiSettings": {"modelLastUsedAt": {}}}`, nil},
		{"members absent", `{}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parserkit.Problems
			state, _ := settingsParts(decoded(t, test.document), 9, &problems)
			standingTexts(state, 9, &problems)
			if got := messages(problems); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}
