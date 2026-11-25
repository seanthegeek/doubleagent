package relocation

import "testing"

// repeatedValues returns n assignments of CODEX_HOME, one a line, with a hit
// on each line.
func repeatedValues(n int) (fileValues, []lineName) {
	var values fileValues
	var hits []lineName
	for line := 1; line <= n; line++ {
		values.Assignments = append(values.Assignments,
			assignment{Line: line, Name: "CODEX_HOME", Value: "/x"})
		hits = append(hits, lineName{Line: line, Name: "CODEX_HOME"})
	}
	return values, hits
}

// Indexing looks at each assignment once and each hit makes one lookup, so
// the work doubles exactly with the assignments; a scan of every assignment
// for each hit would make it grow by four.
func TestSelectingEachHitsValueGrowsLinearlyWithTheAssignments(t *testing.T) {
	work := func(n int) int {
		values, hits := repeatedValues(n)
		index := values.index()
		for _, hit := range hits {
			if value, reason := index.selectValue(hit.Line, hit.Name, ""); value != "/x" || reason != "" {
				t.Fatalf("line %d: selectValue = %q, %q", hit.Line, value, reason)
			}
		}
		return index.Work
	}
	if small, large := work(2000), work(4000); large != 2*small {
		t.Errorf("work %d for 2000 assignments and %d for 4000, want twice", small, large)
	}
}

func TestSelectValueTakesTheLastAssignmentOfALineAndName(t *testing.T) {
	values := fileValues{Assignments: []assignment{
		{Line: 3, Name: "CODEX_HOME", Value: "/first"},
		{Line: 3, Name: "GEMINI_HOME", Value: "/other"},
		{Line: 4, Name: "CODEX_HOME", Value: "/next line"},
		{Line: 3, Name: "CODEX_HOME", Value: "/last"},
	}}
	index := values.index()
	cases := []struct {
		line       int
		name, want string
	}{
		{3, "CODEX_HOME", "/last"},
		{3, "GEMINI_HOME", "/other"},
		{4, "CODEX_HOME", "/next line"},
		{5, "CODEX_HOME", ""},
	}
	for _, testCase := range cases {
		if got, reason := index.selectValue(testCase.line, testCase.name, ""); got != testCase.want || reason != "" {
			t.Errorf("line %d %s: got %q, %q; want %q", testCase.line, testCase.name, got, reason, testCase.want)
		}
	}
}
