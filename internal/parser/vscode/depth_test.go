package vscode

import (
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// nested returns n arrays, each the only item of the one around it.
func nested(n int) string {
	return strings.Repeat("[", n) + strings.Repeat("]", n)
}

// zeros returns n path keys of 0, each the first item of an array.
func zeros(n int) string {
	return strings.Repeat(",0", n)
}

// writeLog writes the lines as a mutation log under a temporary directory
// and returns its path.
func writeLog(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// tooDeep is the reason a record that would nest the state past the
// reader's limit is not applied.
func tooDeep() string {
	return "mutation not applied (the record nests the state deeper than " + strconv.Itoa(jsonvalue.MaxDepth) +
		" levels)"
}

// A depthCase is a record applied to a state of one request beside "a",
// deep arrays each the only item of the one around it, and whether the
// reader's limit refuses it.
type depthCase struct {
	name    string
	deep    int
	record  string
	refused bool
}

// depthCases returns the cases of TestARecordThatWouldNestTheStatePastTheLimitIsRefused.
// With "a" 5,000 arrays deep, a path of 5,001 keys names the place inside
// the innermost, so what a record puts there starts at level 5,002.
func depthCases() []depthCase {
	inside := `["a"` + zeros(5000) + `]`
	return []depthCase{
		{"a set reaching 10000 levels is applied", 5000, `{"kind":1,"k":` + inside + `,"v":` + nested(4999) + `}`, false},
		{"a set reaching 10001 levels is refused", 5000, `{"kind":1,"k":` + inside + `,"v":` + nested(5000) + `}`, true},
		{"a push of an array reaching 10000 levels is applied", 5000,
			`{"kind":2,"k":` + inside + `,"v":` + nested(4999) + `}`, false},
		{"a push of an array reaching 10001 levels is refused", 5000,
			`{"kind":2,"k":` + inside + `,"v":` + nested(5000) + `}`, true},
		{"a push of one value reaching 10000 levels is applied", 5000,
			`{"kind":2,"k":` + inside + `,"v":{"x":` + nested(4997) + `}}`, false},
		{"a push of one value reaching 10001 levels is refused", 5000,
			`{"kind":2,"k":` + inside + `,"v":{"x":` + nested(4998) + `}}`, true},
		{"an empty array made at level 10000 is applied", 9998, `{"kind":2,"k":["a"` + zeros(9998) + `],"v":[]}`, false},
		{"a push whose path alone passes the limit is refused", 9998,
			`{"kind":2,"k":["a"` + zeros(9999) + `],"v":[]}`, true},
	}
}

// Each record that sets or pushes a value is refused when the value would
// end deeper than a decoded document may nest, and the state before it is
// kept: its request's row still appears.
func TestARecordThatWouldNestTheStatePastTheLimitIsRefused(t *testing.T) {
	for _, test := range depthCases() {
		t.Run(test.name, func(t *testing.T) {
			first := `{"kind":0,"v":{"requests":[{"message":"kept"}],"a":` + nested(test.deep) + `}}`
			var problems parser.Problems
			state := replay(writeLog(t, []string{first, test.record}), requestLines{}, &problems, parser.Work{})
			var messages []string
			for _, problem := range problems {
				messages = append(messages, "line "+strconv.Itoa(problem.Line)+": "+problem.Message)
			}
			var want []string
			if test.refused {
				want = []string{"line 2: " + tooDeep()}
			}
			if !slices.Equal(messages, want) {
				t.Errorf("problems %q, want %q", messages, want)
			}
			if _, err := jsonvalue.DecodeString(state.JSON()); err != nil {
				t.Errorf("the state is deeper than a document may be: %v", err)
			}
			if text := state.Get("requests").Index(0).Get("message").StringValue(); text != "kept" {
				t.Errorf("the first request's message is %q, want the state before the record kept", text)
			}
		})
	}
}

// graftLog returns a log whose first line nests "deep" depth arrays and
// whose later lines each set depth more arrays inside the innermost, so
// grafts lines would nest the state grafts*depth levels.
func graftLog(depth, grafts int) []string {
	lines := []string{`{"kind":0,"v":{"requests":[],"deep":` + nested(depth) + `}}`}
	for graft := 1; graft < grafts; graft++ {
		lines = append(lines, `{"kind":1,"k":["deep"`+zeros(graft*depth)+`],"v":`+nested(depth)+`}`)
	}
	return lines
}

// A log of records each nesting the state deeper than the last is refused
// record by record, and the state, never deeper than the limit, is read
// and written within a small stack: 100,000 levels would need several
// times the 16 MiB allowed here.
func TestALogNestingTheStateEverDeeperStaysWithinTheLimit(t *testing.T) {
	defer debug.SetMaxStack(debug.SetMaxStack(16 << 20))
	const depth, grafts = 5000, 20
	var problems parser.Problems
	state := replay(writeLog(t, graftLog(depth, grafts)), requestLines{}, &problems, parser.Work{})
	var lines []int
	for _, problem := range problems {
		if problem.Message != tooDeep() {
			t.Fatalf("line %d: problem %q, want %q", problem.Line, problem.Message, tooDeep())
		}
		lines = append(lines, problem.Line)
	}
	if want := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}; !slices.Equal(lines, want) {
		t.Errorf("problems at lines %v, want %v", lines, want)
	}
	if got, want := state.JSON(), `{"requests":[],"deep":`+nested(depth)+`}`; got != want {
		t.Errorf("the state is not the first line's: %d bytes, want %d", len(got), len(want))
	}
}
