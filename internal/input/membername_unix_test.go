//go:build !windows

package input

import (
	"strings"
	"testing"
)

// longPartReason is the reason a member is skipped whose name holds a part
// longer than the file system stores: the system refuses it for its length.
const longPartReason = reasonTooLong

// A part longer than the file system stores and a whole path longer than
// the system's file calls take give the same error, and the reason fits
// both.
func TestAMemberNameTooLongForTheSystemIsSkippedForItsLength(t *testing.T) {
	tests := []struct {
		name, member string
	}{
		{"a part longer than the file system stores", "alice/.claude/q/" + strings.Repeat("n", 300) + ".jsonl"},
		{"a whole path longer than the system takes, each part valid",
			"alice/.claude/" + strings.Repeat(strings.Repeat("p", 200)+"/", 25) + "a.jsonl"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collection, err := openArchiveOf(t, "c.tar", tarBytes(t, false,
				tarMember{name: test.member, typeflag: '0', body: "x"},
				tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"}))
			opened(t, err)
			wantProblem(t, collection, "member "+test.member+" not extracted: "+reasonTooLong)
			wantSlice(t, "artifacts", rels(collection), []string{".claude/x"})
		})
	}
}
