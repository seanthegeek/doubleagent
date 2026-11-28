package vscode

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// problemMessages returns the messages of problems, in order.
func problemMessages(problems parserkit.Problems) []string {
	var messages []string
	for _, problem := range problems {
		messages = append(messages, problem.Message)
	}
	return messages
}

// parsedRows writes text at rel under a new home and returns the rows the
// parser gives it.
func parsedRows(t *testing.T, rel, text string) []timeline.Row {
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
	var rows []timeline.Row
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// rowTexts returns the texts of rows, in order.
func rowTexts(rows []timeline.Row) []string {
	texts := make([]string, 0, len(rows))
	for _, row := range rows {
		texts = append(texts, row.Text)
	}
	return texts
}

func TestARecentSnapshotOrEntriesOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".config/Code/User/workspaceStorage/h/chatEditingSessions/k/state.json"
	const snapshotWrong = "parser: recentSnapshot is not a JSON object"
	const entriesWrong = "parser: recentSnapshot.entries is not a JSON array"
	tests := []struct {
		name, snapshot string
		want           []string
	}{
		{"recentSnapshot as a string", `"x"`, []string{snapshotWrong}},
		{"recentSnapshot as a number", `3`, []string{snapshotWrong}},
		{"recentSnapshot as an array", `[{"entries": []}]`, []string{snapshotWrong}},
		{"recentSnapshot null", `null`, nil},
		{"recentSnapshot empty", `{}`, nil},
		{"entries as a string", `{"entries": "x"}`, []string{entriesWrong}},
		{"entries as a number", `{"entries": 3}`, []string{entriesWrong}},
		{"entries as an object", `{"entries": {"resource": "file:///a"}}`, []string{entriesWrong}},
		{"entries null", `{"entries": null}`, nil},
		{"entries empty", `{"entries": []}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := rowTexts(parsedRows(t, rel, `{"version": 2, "recentSnapshot": `+test.snapshot+`}`))
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("recentSnapshot absent", func(t *testing.T) {
		if got := rowTexts(parsedRows(t, rel, `{"version": 2}`)); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestASessionOrRequestsOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".config/Code/User/globalStorage/emptyWindowChatSessions/s.json"
	const start = "session start:"
	const requestsWrong = "parser: requests is not a JSON array"
	const sessionWrong = "parser: s.json is not a JSON object"
	tests := []struct {
		name, text string
		want       []string
	}{
		{"requests as a string", `{"sessionId": "s", "requests": "x"}`, []string{start, requestsWrong}},
		{"requests as a number", `{"sessionId": "s", "requests": 3}`, []string{start, requestsWrong}},
		{"requests as an object", `{"sessionId": "s", "requests": {"message": "hi"}}`, []string{start, requestsWrong}},
		{"requests null", `{"sessionId": "s", "requests": null}`, []string{start}},
		{"requests empty", `{"sessionId": "s", "requests": []}`, []string{start}},
		{"requests absent", `{"sessionId": "s"}`, []string{start}},
		{"the session as an array", `[{"sessionId": "s"}]`, []string{sessionWrong}},
		{"the session as a string", `"s"`, []string{sessionWrong}},
		{"the session as a number", `3`, []string{sessionWrong}},
		{"the session null", `null`, nil},
		{"the session empty", `{}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rowTexts(parsedRows(t, rel, test.text)); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestReplayedRequestsThatAreNotAnArrayAreReportedAtTheLineThatSetThem(t *testing.T) {
	const rel = ".config/Code/User/globalStorage/emptyWindowChatSessions/s.jsonl"
	rows := parsedRows(t, rel, "\n"+`{"kind": 0, "v": {"sessionId": "s", "requests": "x"}}`+"\n")
	var lines []int
	for _, row := range rows {
		lines = append(lines, row.SourceLine)
	}
	want := []string{"session start:", "parser: requests is not a JSON array"}
	if got := rowTexts(rows); !slices.Equal(got, want) || !slices.Equal(lines, []int{2, 2}) {
		t.Errorf("rows %q at lines %v, want %q at lines [2 2]", got, lines, want)
	}
}

func TestACopilotStateMemberOfTheWrongKindIsReported(t *testing.T) {
	const stateWrong = "ItemTable value for GitHub.copilot-chat is not a JSON object"
	const tasksWrong = "github.copilot.cloudAgent.ownedTasks is not a JSON object"
	const reposWrong = "userSelectedRepositories is not a JSON array"
	tests := []struct {
		name, state string
		want        []string
	}{
		{"the value as a string", `"x"`, []string{stateWrong}},
		{"the value as a number", `3`, []string{stateWrong}},
		{"the value as an array", `[]`, []string{stateWrong}},
		{"the value null", `null`, nil},
		{"the value empty", `{}`, nil},
		{"ownedTasks as a string", `{"github.copilot.cloudAgent.ownedTasks": "x"}`, []string{tasksWrong}},
		{"ownedTasks as a number", `{"github.copilot.cloudAgent.ownedTasks": 3}`, []string{tasksWrong}},
		{"ownedTasks as an array", `{"github.copilot.cloudAgent.ownedTasks": [{"recordedAt": 1}]}`, []string{tasksWrong}},
		{"ownedTasks null and empty", `{"github.copilot.cloudAgent.ownedTasks": null, "userSelectedRepositories": []}`, nil},
		{"userSelectedRepositories as a string", `{"userSelectedRepositories": "x"}`, []string{reposWrong}},
		{"userSelectedRepositories as a number", `{"userSelectedRepositories": 3}`, []string{reposWrong}},
		{"userSelectedRepositories as an object", `{"userSelectedRepositories": {"name": "o/r"}}`, []string{reposWrong}},
		{"userSelectedRepositories null and ownedTasks empty",
			`{"github.copilot.cloudAgent.ownedTasks": {}, "userSelectedRepositories": null}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parserkit.Problems
			var rows []string
			New().copilotStateRows(&input.Artifact{}, decoded(t, test.state), &problems,
				func(row timeline.Row) { rows = append(rows, row.Text) })
			if !slices.Equal(problemMessages(problems), test.want) || len(rows) != 0 {
				t.Errorf("problems %q and rows %q, want problems %q and no rows", problemMessages(problems), rows, test.want)
			}
		})
	}
	t.Run("a value that is not JSON is reported once", func(t *testing.T) {
		notJSON := "ItemTable value for GitHub.copilot-chat is not valid JSON (x)"
		problems := parserkit.Problems{{Message: notJSON}}
		New().copilotStateRows(&input.Artifact{}, decoded(t, `"{broken"`), &problems, func(timeline.Row) {})
		if !slices.Equal(problemMessages(problems), []string{notJSON}) {
			t.Errorf("problems %q, want only %q", problemMessages(problems), notJSON)
		}
	})
}

func TestAUsageValueThatIsNotAnArrayIsReported(t *testing.T) {
	const key = "github-octocat-usages"
	const wrong = "ItemTable value for github-octocat-usages is not a JSON array"
	tests := []struct {
		name, value string
		want        []string
	}{
		{"a string", `"x"`, []string{wrong}},
		{"a number", `3`, []string{wrong}},
		{"an object", `{"extensionId": "GitHub.copilot-chat"}`, []string{wrong}},
		{"null", `null`, nil},
		{"empty", `[]`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parserkit.Problems
			var rows []string
			New().sourceHostRows(&input.Artifact{}, []usage{{key: key, value: decoded(t, test.value)}}, &problems,
				func(row timeline.Row) { rows = append(rows, row.Text) })
			if !slices.Equal(problemMessages(problems), test.want) || len(rows) != 0 {
				t.Errorf("problems %q and rows %q, want problems %q and no rows", problemMessages(problems), rows, test.want)
			}
		})
	}
}
