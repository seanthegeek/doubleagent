package crush

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
)

// A role other than user or assistant stands before the text of each of
// its message's text parts, so a role over the limit is cut in every one
// of them, with one problem for the message: no row holds it whole, and
// twice the parts cost twice the bytes, never the role's length again. The
// databases are written by testdata/fixtures.py.
func TestALongRoleIsCutInEveryTextPartOfItsMessage(t *testing.T) {
	role := strings.Repeat("r", 5000)
	var sizes []int
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d text parts", count), func(t *testing.T) {
			rows := parseDatabase(t, fmt.Sprintf("long_role_%d.db", count))
			parts, problems, whole, size := 0, 0, 0, 0
			for _, text := range rows {
				size += len(text)
				if strings.HasPrefix(text, "rrrr") && strings.Contains(text, ": part ") {
					parts++
				}
				if text == parser.ProblemPrefix+parser.CarriedProblem("role", 1) {
					problems++
				}
				if strings.Contains(text, role) {
					whole++
				}
			}
			sizes = append(sizes, size)
			if parts != count || problems != 1 || whole != 0 {
				t.Errorf("%d part rows, %d problems and %d whole roles, want %d, 1 and 0", parts, problems, whole,
					count)
			}
		})
	}
	if growth := sizes[1] - sizes[0]; growth > 30*(2*parser.CarriedLimit+1024) {
		t.Errorf("30 more parts cost %d bytes", growth)
	}
}
