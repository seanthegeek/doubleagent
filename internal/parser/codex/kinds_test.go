package codex

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// globalStateProblems writes text as a home's .codex-global-state.json,
// parses it and returns the texts of its problem rows.
func globalStateProblems(t *testing.T, text string) []string {
	t.Helper()
	const rel = ".codex/.codex-global-state.json"
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var problems []string
	err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
			problems = append(problems, row.Text)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

// globalStateCase is one member of the global state that the app writes
// as an object: the document with its value in place of "@" and the
// problem a wrong kind gives.
type globalStateCase struct {
	name, document, want string
}

// globalStateCases are the members of the global state read for rows.
func globalStateCases() []globalStateCase {
	inAtom := func(key string) string { return `{"electron-persisted-atom-state": {"` + key + `": @}}` }
	return []globalStateCase{
		{atomKey, `{"electron-persisted-atom-state": @}`, "parser: electron-persisted-atom-state is not a JSON object"},
		{promptHistoryKey, inAtom(promptHistoryKey), "parser: prompt-history is not a JSON object"},
		{descriptionsKey, inAtom(descriptionsKey), "parser: thread-descriptions-v1 is not a JSON object"},
		{activityKey, inAtom(activityKey), "parser: thread-user-activity-times-v1 is not a JSON object"},
		{permissionsKey, inAtom(permissionsKey), "parser: heartbeat-thread-permissions-by-id is not a JSON object"},
		{hostSettingKeys[0], inAtom(hostSettingKeys[0]), "parser: agent-mode-by-host-id is not a JSON object"},
		{hostSettingKeys[1], inAtom(hostSettingKeys[1]),
			"parser: preferred-non-full-access-agent-mode-by-host-id is not a JSON object"},
		{outputDirsKey, `{"thread-projectless-output-directories": @}`,
			"parser: thread-projectless-output-directories is not a JSON object"},
	}
}

func TestAGlobalStateMemberThatIsNotAnObjectIsReported(t *testing.T) {
	for _, test := range globalStateCases() {
		t.Run(test.name, func(t *testing.T) {
			// A string holding text that is not JSON, or JSON that is not an
			// object, is of the wrong kind too.
			for _, value := range []string{`"x"`, `"[1]"`, `7`, `[1]`} {
				got := globalStateProblems(t, strings.ReplaceAll(test.document, "@", value))
				if want := []string{test.want}; !slices.Equal(got, want) {
					t.Errorf("as %s: problem rows %q, want %q", value, got, want)
				}
			}
			for _, value := range []string{`null`, `{}`, `"{}"`, `"null"`} {
				if got := globalStateProblems(t, strings.ReplaceAll(test.document, "@", value)); len(got) != 0 {
					t.Errorf("as %s: problem rows %q, want none", value, got)
				}
			}
		})
	}
	if got := globalStateProblems(t, `{}`); len(got) != 0 {
		t.Errorf("absent members: problem rows %q, want none", got)
	}
}

func TestAPromptHistoryListThatIsNotAnArrayIsReported(t *testing.T) {
	const document = `{"electron-persisted-atom-state": {"prompt-history": {"t1": @}}}`
	for _, value := range []string{`"x"`, `7`, `{"markdown": "hi"}`} {
		got := globalStateProblems(t, strings.ReplaceAll(document, "@", value))
		if want := []string{"parser: prompt-history.t1 is not a JSON array"}; !slices.Equal(got, want) {
			t.Errorf("as %s: problem rows %q, want %q", value, got, want)
		}
	}
	for _, value := range []string{`null`, `[]`} {
		if got := globalStateProblems(t, strings.ReplaceAll(document, "@", value)); len(got) != 0 {
			t.Errorf("as %s: problem rows %q, want none", value, got)
		}
	}
}

func TestAThreadItemListThatIsNotAnArrayIsReported(t *testing.T) {
	tests := []struct {
		name, item, want string
	}{
		{"a user message's content", `{"type": "userMessage", "content": @}`, "content"},
		{"a reasoning's summary", `{"type": "reasoning", "summary": @, "content": ["thought"]}`, "summary"},
		{"a reasoning's content", `{"type": "reasoning", "summary": [], "content": @}`, "content"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range []string{`"x"`, `7`, `{"type": "text", "text": "hi"}`, `null`, `[]`} {
				item, err := jsonvalue.DecodeString(strings.ReplaceAll(test.item, "@", value))
				if err != nil {
					t.Fatal(err)
				}
				var problems parser.Problems
				threadItemRows(item, sqlitefile.Row{Rowid: 9}, &problems)
				var want parser.Problems
				if value != `null` && value != `[]` {
					want = parser.Problems{{Line: 9, Message: test.want + " is not a JSON array"}}
				}
				if !slices.Equal(problems, want) {
					t.Errorf("as %s: problems %v, want %v", value, problems, want)
				}
			}
		})
	}
	var problems parser.Problems
	if threadItemRows(jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("userMessage")}),
		sqlitefile.Row{Rowid: 9}, &problems); len(problems) != 0 {
		t.Errorf("an absent content: problems %v, want none", problems)
	}
}

// rolloutProblems writes lines as a rollout of a home, parses it and
// returns the texts of its problem rows.
func rolloutProblems(t *testing.T, lines ...string) []string {
	t.Helper()
	const rel = ".codex/sessions/2026/01/02/rollout-2026-01-02T00-00-00-0198f0a1-0000-7000-8000-000000000001.jsonl"
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var problems []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
			problems = append(problems, row.Text)
		}
	}); err != nil {
		t.Fatal(err)
	}
	return problems
}

func TestARolloutPayloadThatIsNotAnObjectIsReported(t *testing.T) {
	tests := []struct {
		name, payload string
		want          []string
	}{
		{"an object", `{"type":"task_started"}`, nil},
		{"an empty object", `{}`, nil},
		{"null", `null`, nil},
		{"a string", `"x"`, []string{"parser: payload is not a JSON object"}},
		{"a number", `5`, []string{"parser: payload is not a JSON object"}},
		{"an array", `[{"type":"task_started"}]`, []string{"parser: payload is not a JSON object"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := rolloutProblems(t, `{"timestamp":"2026-01-02T00:00:00Z","type":"event_msg","payload":`+test.payload+`}`)
			if !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}
