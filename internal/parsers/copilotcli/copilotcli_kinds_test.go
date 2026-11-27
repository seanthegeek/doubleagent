package copilotcli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sessionDir is the directory of a session under a home.
const sessionDir = ".copilot/session-state/0123456789abcdef0123456789abcdef0123/"

// parseWith writes text at rel under a new home and returns the texts of
// the rows the parser gives it with options.
func parseWith(t *testing.T, rel, text string, options parserkit.Options) []string {
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
	if err := New().Parse(artifact, options, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// kindCase is one value of a member and the rows the file holding it
// gives.
type kindCase struct {
	name, value string
	want        []string
}

// runKindCases parses, for each case, the text that document returns for
// the case's value, written at rel.
func runKindCases(t *testing.T, rel string, document func(string) string, tests []kindCase) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseWith(t, rel, document(test.value), parserkit.Options{IncludeThinking: true})
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestASnapshotsThatIsNotAnArrayIsReported(t *testing.T) {
	const wrong = "parser: snapshots is not a JSON array"
	runKindCases(t, sessionDir+"rewind-file-snapshots/index.json",
		func(value string) string { return `{"schema": 1, "snapshots": ` + value + `}` },
		[]kindCase{
			{"a string", `"x"`, []string{wrong}},
			{"a number", `3`, []string{wrong}},
			{"an object", `{"userMessage": "hi"}`, []string{wrong}},
			{"null", `null`, nil},
			{"empty", `[]`, nil},
		})
}

func TestACommandHistoryThatIsNotAnArrayIsReported(t *testing.T) {
	const wrong = "parser: commandHistory is not a JSON array"
	runKindCases(t, ".copilot/command-history-state.json",
		func(value string) string { return `{"commandHistory": ` + value + `}` },
		[]kindCase{
			{"a string", `"run tests"`, []string{wrong}},
			{"a number", `3`, []string{wrong}},
			{"an object", `{"a": "run tests"}`, []string{wrong}},
			{"null", `null`, nil},
			{"empty", `[]`, nil},
			{"entries that are not text among prompts", `["run tests", 3, {}]`,
				[]string{"run tests", "parser: commandHistory holds 2 entries that are not JSON strings"}},
		})
}

func TestAPermissionsLocationMemberOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".copilot/permissions-config.json"
	runKindCases(t, rel, func(value string) string { return `{"locations": ` + value + `}` },
		[]kindCase{
			{"locations as a string", `"x"`, []string{"parser: locations is not a JSON object"}},
			{"locations as a number", `3`, []string{"parser: locations is not a JSON object"}},
			{"locations as an array", `[]`, []string{"parser: locations is not a JSON object"}},
			{"locations null", `null`, nil},
			{"locations empty", `{}`, nil},
			{"a location as a string", `{"/p": "x"}`, []string{"parser: locations./p is not a JSON object"}},
			{"a location as an array", `{"/p": []}`, []string{"parser: locations./p is not a JSON object"}},
			{"tool_approvals as an object", `{"/p": {"tool_approvals": {"kind": "write"}}}`,
				[]string{"parser: locations./p.tool_approvals is not a JSON array"}},
			{"allowed_directories as a string", `{"/p": {"allowed_directories": "/q"}}`,
				[]string{"parser: locations./p.allowed_directories is not a JSON array"}},
			{"allowed_directories as a number", `{"/p": {"allowed_directories": 3}}`,
				[]string{"parser: locations./p.allowed_directories is not a JSON array"}},
			{"lists null and empty", `{"/p": {"tool_approvals": null, "allowed_directories": []}}`, nil},
			{"a location empty", `{"/p": {}}`, nil},
		})
	t.Run("locations absent", func(t *testing.T) {
		if got := parseWith(t, rel, `{}`, parserkit.Options{}); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestAURLListThatIsNotAnArrayIsReported(t *testing.T) {
	runKindCases(t, ".copilot/settings.json", func(value string) string { return `{` + value + `}` },
		[]kindCase{
			{"allowedUrls as a string", `"allowedUrls": "x"`, []string{"parser: allowedUrls is not a JSON array"}},
			{"allowedUrls as a number", `"allowedUrls": 3`, []string{"parser: allowedUrls is not a JSON array"}},
			{"deniedUrls as an object", `"deniedUrls": {"a": 1}`, []string{"parser: deniedUrls is not a JSON array"}},
			{"lists null and empty", `"allowedUrls": null, "deniedUrls": []`, nil},
			{"lists absent", `"other": 1`, nil},
			{"entries that are not strings", `"allowedUrls": ["https://a.example/", {"url": "https://b.example/"}, ` +
				`["https://c.example/"]], "deniedUrls": [7]`,
				[]string{"permission setting: allowedUrls=https://a.example/ (from .copilot/settings.json, file mtime)",
					"parser: allowedUrls holds 2 entries that are not JSON strings",
					"parser: deniedUrls holds 1 entry that is not a JSON string"}},
		})
}

func TestAnEventMemberOfTheWrongKindIsReported(t *testing.T) {
	event := func(data string) string {
		return `{"type": "assistant.message", "timestamp": "2026-10-01T09:00:00.000Z", "data": ` + data + `}` + "\n"
	}
	runKindCases(t, sessionDir+"events.jsonl", event, []kindCase{
		{"data as a string", `"x"`, []string{"parser: data is not a JSON object"}},
		{"data as a number", `3`, []string{"parser: data is not a JSON object"}},
		{"data as an array", `[]`, []string{"parser: data is not a JSON object"}},
		{"data null", `null`, nil},
		{"data empty", `{}`, nil},
		{"toolRequests as a string", `{"toolRequests": "x"}`, []string{"parser: toolRequests is not a JSON array"}},
		{"toolRequests as a number", `{"toolRequests": 3}`, []string{"parser: toolRequests is not a JSON array"}},
		{"toolRequests as an object", `{"toolRequests": {"name": "bash"}}`,
			[]string{"parser: toolRequests is not a JSON array"}},
		{"toolRequests null", `{"toolRequests": null}`, nil},
		{"toolRequests empty", `{"toolRequests": []}`, nil},
		{"reasoningBlocks as an array", `{"reasoningBlocks": []}`,
			[]string{"parser: reasoningBlocks is not a JSON object"}},
		{"blocks as a string", `{"reasoningBlocks": {"blocks": "x"}}`,
			[]string{"parser: reasoningBlocks.blocks is not a JSON array"}},
		{"summary as a number", `{"reasoningBlocks": {"blocks": [{"summary": 3}]}}`,
			[]string{"parser: summary is not a JSON array"}},
		{"reasoning null and empty", `{"reasoningBlocks": {"blocks": [{"summary": []}, {"summary": null}]}}`, nil},
		{"summary as a number in three blocks", `{"reasoningBlocks": {"blocks": [{"summary": 3}, {"summary": 4}, ` +
			`{"summary": "x"}]}}`, []string{"parser: summary is not a JSON array in 3 entries"}},
		{"blocks that are not objects", `{"reasoningBlocks": {"blocks": ["a", 7, {"summary": []}]}}`,
			[]string{"parser: reasoningBlocks.blocks holds 2 entries that are not JSON objects"}},
		{"summaries that are not objects across blocks", `{"reasoningBlocks": {"blocks": [` +
			`{"summary": [{}, "a", 9]}, {"summary": [true, {}]}]}}`,
			[]string{"parser: summary holds 3 entries that are not JSON objects"}},
	})
}

func TestASidebarFileNamedInAnotherCaseGivesItsSessionsDirectory(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".copilot", "sidebar-sessions-state", strings.Repeat("A", 64)+".JSON")
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(`{"cwd": "/w", "sessionIds": ["s1"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{Rel: sessionDir + "events.jsonl", Home: &input.Home{DiskPath: home}}
	if got := New().sidebarJoin(artifact); got["s1"] != "/w" {
		t.Errorf("got %q, want s1 in /w", got)
	}
}

func TestAnEventProblemComesAfterTheEventsRows(t *testing.T) {
	text := `{"type": "assistant.message", "data": {"content": "done", "toolRequests": "x"}}` + "\n"
	got := parseWith(t, sessionDir+"events.jsonl", text, parserkit.Options{})
	want := []string{"done", "parser: toolRequests is not a JSON array"}
	if !slices.Equal(got, want) {
		t.Errorf("rows %q, want %q", got, want)
	}
}
