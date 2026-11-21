package parser

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// groupBytes returns the bytes of every string of the rows one parse
// writes: a group of two stored rows whose fields are each longer than
// CarriedLimit, the second's own row tagged "later", whose text names one
// long shared text, and members members taking takes, attached to the
// first stored row or, with later set, to the second, and written out as a
// run writes them (timeline.Resolve).
func groupBytes(takes timeline.Takes, members int, later bool) int {
	long := strings.Repeat("v", 3*CarriedLimit)
	artifact := &input.Artifact{Original: "/home/user/file", Home: &input.Home{Host: "host", User: "user"}}
	var rows []timeline.Row
	_, _ = Within(artifact, func(emit Emit) error {
		group := timeline.NewGroup()
		shared := timeline.NewSharedText(long)
		for _, name := range []string{"first", "later"} {
			parent := timeline.Row{TimestampUTC: name + long, SessionID: name + long, ProjectPath: "/" + name + long,
				GitBranch: name + long, Model: name + long, ToolName: name, SourceFile: "file", SourceLine: 1}
			var text timeline.TextBuilder
			text.Write(name + " ")
			text.Share(shared, "[shared]", func(*timeline.Row, bool) string { return "[shown above]" })
			text.Set(&parent)
			group.Parent(&parent)
			emit(parent)
		}
		for i := range members {
			member := timeline.Row{Text: "member", SourceFile: "file", SourceLine: 2 + i}
			group.Member(&member, takes)
			emit(member)
		}
		return nil
	}, func(row timeline.Row) { rows = append(rows, row) })
	var choice timeline.Choice
	if later {
		for _, row := range rows {
			if row.ToolName == "later" {
				choice = timeline.Choose([]timeline.Row{row}, timeline.TestedText)
			}
		}
	}
	rows = timeline.Attach(rows, choice)
	timeline.Resolve(rows)
	total := 0
	for _, row := range rows {
		for _, field := range row.Strings() {
			total += len(field)
		}
	}
	return total
}

// A member takes each field of the stored row its group goes with as the
// kit cut it on that row, whichever row that is, and a text that names a
// shared text as a reference to it: each of twice the members adds at most
// a cut field's length and a pointer, never the whole value or text.
func TestAMemberTakesEachFieldOfItsParentCut(t *testing.T) {
	flags := []struct {
		name  string
		takes timeline.Takes
	}{{"time", timeline.TakeTime}, {"session", timeline.TakeSession}, {"project", timeline.TakeProject},
		{"branch", timeline.TakeBranch}, {"model", timeline.TakeModel}, {"text", timeline.TakeText},
		{"source line", timeline.TakeSourceLine}}
	for _, flag := range flags {
		for _, later := range []bool{false, true} {
			t.Run(flag.name+" later "+strconv.FormatBool(later), func(t *testing.T) {
				small, large := groupBytes(flag.takes, 50, later), groupBytes(flag.takes, 100, later)
				if perMember := (large - small) / 50; perMember > CarriedLimit+64 {
					t.Errorf("50 members wrote %d bytes and 100 wrote %d: %d a member", small, large, perMember)
				}
			})
		}
	}
}

// A member's cut of a field it takes from its parent is not counted: the
// problem row counts the parents' own rows, here two, and a member's cut
// of a field of its own is counted.
func TestAMembersCutOfATakenFieldIsNotCounted(t *testing.T) {
	long := "/" + strings.Repeat("v", 3*CarriedLimit)
	artifact := &input.Artifact{Original: "/home/user/file", Home: &input.Home{Host: "host", User: "user"}}
	var problems []string
	_, _ = Within(artifact, func(emit Emit) error {
		group := timeline.NewGroup()
		for _, name := range []string{"a", "b"} {
			parent := timeline.Row{ProjectPath: long + name, Text: name, SourceFile: "file"}
			group.Parent(&parent)
			emit(parent)
		}
		for i := range 5 {
			member := timeline.Row{Text: "member", SourceFile: "file", SessionID: strings.Repeat("s", i%2*2*CarriedLimit)}
			group.Member(&member, timeline.TakeProject)
			emit(member)
		}
		return nil
	}, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, ProblemPrefix) {
			problems = append(problems, row.Text)
		}
	})
	want := []string{ProblemPrefix + CarriedProblem("project_path", 2), ProblemPrefix + CarriedProblem("session_id", 2)}
	if !slices.Equal(problems, want) {
		t.Errorf("problems %q, want %q", problems, want)
	}
}
