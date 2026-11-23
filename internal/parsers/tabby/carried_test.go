package tabby

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// A user's label is written into every thread the user owns, and an
// integration's name and kind into every repository row of it, so a value
// over the limit is cut in each of those rows, with one problem per value
// for the database: the value is written whole at most once, in its own
// row, and twice the rows cost twice the bytes, never the value's length
// again. The databases are written by testdata/fixtures.py.
func TestAJoinedLabelNameOrKindIsCutInEveryRowItIsCopiedInto(t *testing.T) {
	const long = 5000
	tests := []struct {
		name, database, rows string
		values               []string
		problems             func(count int) []string
	}{
		{"a user's label", "long_user_%d.db", "thread start", []string{strings.Repeat("n", long)},
			func(count int) []string { return []string{parserkit.CarriedProblem("user label", count)} }},
		{"an integration's name and kind", "long_integration_%d.db", "source host repository",
			[]string{strings.Repeat("n", long), strings.Repeat("k", long)}, func(int) []string {
				return []string{parserkit.CarriedProblem("integration name", 1), parserkit.CarriedProblem("integration kind", 1)}
			}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sizes []int
			for _, count := range []int{10, 40} {
				rows := parseDatabase(t, fmt.Sprintf(test.database, count))
				joined, size := 0, 0
				for _, text := range rows {
					size += len(text)
					if strings.HasPrefix(text, test.rows) {
						joined++
					}
				}
				sizes = append(sizes, size)
				for _, value := range test.values {
					if whole := countContaining(rows, value); whole > 1 {
						t.Errorf("%d rows hold a value whole, want one at most", whole)
					}
				}
				for _, problem := range test.problems(count) {
					if got := countOf(rows, parserkit.ProblemPrefix+problem); got != 1 {
						t.Errorf("%d rows of %q, want 1", got, problem)
					}
				}
				if joined != count {
					t.Errorf("%d rows starting %q, want %d", joined, test.rows, count)
				}
			}
			if growth := sizes[1] - sizes[0]; growth > 30*(2*parserkit.CarriedLimit+1024) {
				t.Errorf("30 more rows cost %d bytes", growth)
			}
		})
	}
}

// countContaining returns how many of the rows hold text.
func countContaining(rows []string, text string) int {
	total := 0
	for _, row := range rows {
		if strings.Contains(row, text) {
			total++
		}
	}
	return total
}

// countOf returns how many of the rows are exactly text.
func countOf(rows []string, text string) int {
	total := 0
	for _, row := range rows {
		if row == text {
			total++
		}
	}
	return total
}
