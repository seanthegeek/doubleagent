package claudecode

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func TestEachPasteCacheFileIsReadOncePerLine(t *testing.T) {
	const placeholders = 2000
	tests := []struct {
		name     string
		hashes   []string
		readable bool
		reads    int
	}{
		{"one file named by every placeholder", []string{"abc"}, true, 1},
		{"two files named in turn", []string{"abc", "def"}, true, 2},
		{"one file that cannot be read", []string{"abc"}, false, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			members := make([]jsonvalue.Member, placeholders)
			var display, want strings.Builder
			for i := range placeholders {
				number, hash := strconv.Itoa(i), test.hashes[i%len(test.hashes)]
				members[i] = jsonvalue.Member{Key: number,
					Value: jsonvalue.NewObject(jsonvalue.Member{Key: "contentHash", Value: jsonvalue.NewString(hash)})}
				display.WriteString("[Pasted text #" + number + "] ")
				if test.readable {
					want.WriteString("text of " + hash + " ")
				} else {
					want.WriteString("[Pasted text #" + number + "] ")
				}
			}
			reads := 0
			var problems parser.Problems
			got := expandPastesFrom(display.String(), jsonvalue.NewObject(members...), &problems,
				func(hash string, read *parser.Problems) (string, bool) {
					reads++
					if !test.readable {
						read.Add(0, hash+".txt could not be read")
						return "", false
					}
					return "text of " + hash, true
				})
			wantProblems := 0
			if !test.readable {
				wantProblems = placeholders
			}
			if got != want.String() || reads != test.reads || len(problems) != wantProblems {
				t.Errorf("%d reads and %d problems, text as wanted %v; want %d reads and %d problems",
					reads, len(problems), got == want.String(), test.reads, wantProblems)
			}
		})
	}
}
