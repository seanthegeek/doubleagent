package zed

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestAThreadsRemoteURLShowsItsCredentialsUnlessWithheld(t *testing.T) {
	const blob = `{"initial_project_snapshot": {"worktree_snapshots": [{"worktree_path": "/w", ` +
		`"git_state": {"remote_url": "https://ci:s3cret@github.com/o/r.git"}}]}}`
	tests := []struct {
		name    string
		setting secrets.Setting
		scheme  string
	}{
		{"shown by default", secrets.Setting{}, "https://ci:s3cret@"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			"https://REDACTED@"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			"https://[withheld]@"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, "https://"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parser.Problems
			context := &thread{rowid: 7, artifact: &input.Artifact{Home: &input.Home{Secrets: test.setting}}}
			var got []string
			New().threadState(context, sqlitefile.Row{Rowid: 7}, decoded(t, blob), &problems,
				func(row timeline.Row) { got = append(got, row.Text) })
			if want := timeline.GitRemotePrefix + test.scheme + "github.com/o/r.git"; !slices.Contains(got, want) {
				t.Errorf("got %q, want a row %q", got, want)
			}
			if test.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "s3cret") {
				t.Errorf("a withheld run shows the password: %q", got)
			}
		})
	}
}
