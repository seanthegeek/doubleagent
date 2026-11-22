package zed

import (
	"fmt"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// threadProblems returns the texts of the problems a thread blob gives,
// read through the thread's state and its messages.
func threadProblems(t *testing.T, blob string) []string {
	t.Helper()
	var problems parser.Problems
	context := &thread{rowid: 7, artifact: &input.Artifact{}}
	value := decoded(t, blob)
	New().threadState(context, sqlitefile.Row{Rowid: 7}, value, &problems, func(timeline.Row) {})
	New().messages(value, 7, &problems)
	var texts []string
	for _, problem := range problems {
		texts = append(texts, problem.Message)
	}
	return texts
}

// wrongKinds returns the values of the kinds a member must not have, each
// with its name, for a member that is an array when `array` is true and an
// object when it is not.
func wrongKinds(array bool) [][2]string {
	other := `[1]`
	if array {
		other = `{"a": 1}`
	}
	return [][2]string{{"a string", `"oops"`}, {"a number", `7`}, {"the other container", other}}
}

func TestALegacyMessageMemberThatIsNotAnArrayIsReported(t *testing.T) {
	for _, member := range []string{"segments", "tool_uses", "tool_results"} {
		for _, wrong := range wrongKinds(true) {
			t.Run(member+" as "+wrong[0], func(t *testing.T) {
				got := threadProblems(t, `{"messages": [{"role": "assistant", "`+member+`": `+wrong[1]+`}]}`)
				want := []string{member + " is not a JSON array"}
				if !slices.Equal(got, want) {
					t.Errorf("problems %q, want %q", got, want)
				}
			})
		}
		for _, silent := range []string{`null`, `[]`} {
			t.Run(member+" as "+silent, func(t *testing.T) {
				if got := threadProblems(t, `{"messages": [{"role": "assistant", "`+member+`": `+silent+`}]}`); len(got) != 0 {
					t.Errorf("problems %q, want none", got)
				}
			})
		}
	}
	t.Run("absent", func(t *testing.T) {
		if got := threadProblems(t, `{"messages": [{"role": "assistant", "text": "x"}]}`); len(got) != 0 {
			t.Errorf("problems %q, want none", got)
		}
	})
}

func TestAThreadStateMemberOfTheWrongKindIsReported(t *testing.T) {
	tests := []struct {
		name, member, blob string
		array              bool
	}{
		{"worktree_snapshots", "initial_project_snapshot.worktree_snapshots",
			`{"initial_project_snapshot": {"worktree_snapshots": %s}}`, true},
		{"write_paths", "sandbox_grants.write_paths", `{"sandbox_grants": {"write_paths": %s}}`, true},
		{"network_hosts", "sandbox_grants.network_hosts", `{"sandbox_grants": {"network_hosts": %s}}`, true},
		{"initial_project_snapshot", "initial_project_snapshot", `{"initial_project_snapshot": %s}`, false},
		{"sandbox_grants", "sandbox_grants", `{"sandbox_grants": %s}`, false},
		{"model", "model", `{"model": %s}`, false},
	}
	for _, test := range tests {
		for _, wrong := range wrongKinds(test.array) {
			t.Run(test.name+" as "+wrong[0], func(t *testing.T) {
				got := threadProblems(t, fmt.Sprintf(test.blob, wrong[1]))
				want := []string{test.member + " is not a JSON " + map[bool]string{true: "array", false: "object"}[test.array]}
				if !slices.Equal(got, want) {
					t.Errorf("problems %q, want %q", got, want)
				}
			})
		}
		empty := `{}`
		if test.array {
			empty = `[]`
		}
		for _, silent := range []string{`null`, empty} {
			t.Run(test.name+" as "+silent, func(t *testing.T) {
				if got := threadProblems(t, fmt.Sprintf(test.blob, silent)); len(got) != 0 {
					t.Errorf("problems %q, want none", got)
				}
			})
		}
	}
	t.Run("absent", func(t *testing.T) {
		if got := threadProblems(t, `{}`); len(got) != 0 {
			t.Errorf("problems %q, want none", got)
		}
	})
}

func TestWorktreeSnapshotsThatAreNotObjectsAreReportedOnce(t *testing.T) {
	got := threadProblems(t, `{"initial_project_snapshot": {"worktree_snapshots": [{"worktree_path": "/w"}, "x", 7]}}`)
	want := []string{"initial_project_snapshot.worktree_snapshots holds 2 entries that are not JSON objects"}
	if !slices.Equal(got, want) {
		t.Errorf("problems %q, want %q", got, want)
	}
}
