package vscode

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// problemTexts returns the texts of the rows that report a problem.
func problemTexts(rows []timeline.Row) []string {
	var texts []string
	for _, text := range rowTexts(rows) {
		if strings.HasPrefix(text, "parser: ") {
			texts = append(texts, text)
		}
	}
	return texts
}

func TestEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	tests := []struct {
		name, rel, text string
		want            []string
	}{
		{"edited file entries", ".config/Code/User/workspaceStorage/h/chatEditingSessions/k/state.json",
			`{"recentSnapshot": {"entries": [{"resource": "file:///a"}, "x", 3]}}`,
			[]string{"parser: recentSnapshot.entries holds 2 entries that are not JSON objects"}},
		{"chat requests", ".config/Code/User/globalStorage/emptyWindowChatSessions/s.json",
			`{"sessionId": "s", "requests": [{"message": {"text": "hi"}}, "x"]}`,
			[]string{"parser: requests holds 1 entry that is not a JSON object"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := problemTexts(parsedRows(t, test.rel, test.text)); !slices.Equal(got, test.want) {
				t.Errorf("problem rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestStateValueEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	t.Run("an account usage list", func(t *testing.T) {
		var problems []string
		var rows []string
		New().sourceHostRows(&input.Artifact{}, []usage{{key: "github-octocat-usages",
			value: decoded(t, `[{"extensionId": "a"}, "x", 1]`)}}, &problems,
			func(row timeline.Row) { rows = append(rows, row.Text) })
		want := []string{"ItemTable value for github-octocat-usages holds 2 entries that are not JSON objects"}
		if !slices.Equal(problems, want) || len(rows) != 1 {
			t.Errorf("problems %q and %d rows, want %q and one row", problems, len(rows), want)
		}
	})
	t.Run("selected repositories", func(t *testing.T) {
		var problems []string
		New().copilotStateRows(&input.Artifact{}, decoded(t, `{"userSelectedRepositories": [{"name": "o/r"}, "x"]}`),
			&problems, func(timeline.Row) {})
		want := []string{"userSelectedRepositories holds 1 entry that is not a JSON object"}
		if !slices.Equal(problems, want) {
			t.Errorf("problems %q, want %q", problems, want)
		}
	})
}
