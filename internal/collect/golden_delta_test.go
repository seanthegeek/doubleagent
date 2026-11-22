package collect_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A delta is a file stored as its differences from the default case's file
// of the same name, so a case over the main image keeps only the lines it
// changes. Each hunk starts with a line naming the base line it follows,
// "@@ after <line>" or "@@ at start", then the base lines it removes, each
// after "-", and the lines it adds, each after "+". The anchor is the first
// line equal to it after the previous hunk, so a change elsewhere in the
// base leaves the delta as it was.

// deltaHeader opens every delta file, to say what it is to a reader.
const deltaHeader = "# the default case's file with these changes: each hunk follows the line it names\n"

// edit is one step of a line diff: a base line kept, removed or a line added.
type edit struct {
	kind byte // ' ', '-' or '+'
	line string
}

// diffLines returns the shortest edit script from base to target, by
// Myers's algorithm (An O(ND) Difference Algorithm and Its Variations, 1986):
// for each number of differences it records how far along base the
// furthest point of each diagonal reaches, then walks the recorded fronts
// back from the end. Each round keeps only the diagonals it can reach.
func diffLines(base, target []string) []edit {
	limit := len(base) + len(target)
	offset := limit + 1
	front := make([]int, 2*limit+3)
	var fronts [][]int
	for differences := 0; differences <= limit; differences++ {
		fronts = append(fronts, slices.Clone(front[offset-differences-1:offset+differences+2]))
		for diagonal := -differences; diagonal <= differences; diagonal += 2 {
			reached := front[offset+diagonal-1] + 1
			if movesDown(front, offset, diagonal, differences) {
				reached = front[offset+diagonal+1]
			}
			for reached < len(base) && reached-diagonal < len(target) && base[reached] == target[reached-diagonal] {
				reached++
			}
			front[offset+diagonal] = reached
			if reached >= len(base) && reached-diagonal >= len(target) {
				return backtrack(base, target, fronts)
			}
		}
	}
	return nil
}

// movesDown reports whether the furthest point on diagonal after a number
// of differences is reached from the diagonal above it, by adding a target
// line, rather than from the one below, by removing a base line. front
// holds diagonal 0 at offset.
func movesDown(front []int, offset, diagonal, differences int) bool {
	return diagonal == -differences ||
		(diagonal != differences && front[offset+diagonal-1] < front[offset+diagonal+1])
}

// backtrack walks the fronts diffLines recorded from the end of both texts
// back to the start and returns the edits in order. The front recorded
// before round d holds diagonals -d-1 to d+1.
func backtrack(base, target []string, fronts [][]int) []edit {
	var reversed []edit
	inBase, inTarget := len(base), len(target)
	for differences := len(fronts) - 1; differences >= 0; differences-- {
		front, offset, diagonal := fronts[differences], differences+1, inBase-inTarget
		previous := diagonal - 1
		if movesDown(front, offset, diagonal, differences) {
			previous = diagonal + 1
		}
		start := front[offset+previous]
		for inBase > start && inTarget > start-previous {
			inBase, inTarget = inBase-1, inTarget-1
			reversed = append(reversed, edit{' ', base[inBase]})
		}
		switch {
		case differences == 0:
		case inBase == start:
			inTarget--
			reversed = append(reversed, edit{'+', target[inTarget]})
		default:
			inBase--
			reversed = append(reversed, edit{'-', base[inBase]})
		}
	}
	slices.Reverse(reversed)
	return reversed
}

// makeDelta returns target as a delta from base, both texts of lines.
func makeDelta(base, target string) string {
	var out strings.Builder
	out.WriteString(deltaHeader)
	anchor, inHunk := "@@ at start", false
	for _, step := range diffLines(splitLines(base), splitLines(target)) {
		if step.kind == ' ' {
			anchor, inHunk = "@@ after "+step.line, false
			continue
		}
		if !inHunk {
			out.WriteString(anchor + "\n")
			inHunk = true
		}
		out.WriteString(string(step.kind) + step.line + "\n")
	}
	return out.String()
}

// errDeltaDoesNotApply is the error of a delta whose anchor or removed line
// the base does not hold where the delta says.
var errDeltaDoesNotApply = errors.New("the delta does not apply to the default case's file")

// applyDelta returns base with the delta's changes.
func applyDelta(base, delta string) (string, error) {
	lines := splitLines(base)
	var out []string
	position := 0
	for number, line := range splitLines(strings.TrimPrefix(delta, deltaHeader)) {
		switch {
		case line == "@@ at start":
			if position != 0 {
				return "", fmt.Errorf("line %d: %w", number+2, errDeltaDoesNotApply)
			}
		case strings.HasPrefix(line, "@@ after "):
			anchor := strings.TrimPrefix(line, "@@ after ")
			found := indexFrom(lines, anchor, position)
			if found < 0 {
				return "", fmt.Errorf("line %d: anchor not found: %w", number+2, errDeltaDoesNotApply)
			}
			out, position = append(out, lines[position:found+1]...), found+1
		case strings.HasPrefix(line, "-"):
			if position >= len(lines) || lines[position] != line[1:] {
				return "", fmt.Errorf("line %d: removed line not found: %w", number+2, errDeltaDoesNotApply)
			}
			position++
		case strings.HasPrefix(line, "+"):
			out = append(out, line[1:])
		default:
			return "", fmt.Errorf("line %d: %q is no delta line: %w", number+2, line, errDeltaDoesNotApply)
		}
	}
	return joinLines(append(out, lines[position:]...)), nil
}

// indexFrom returns the index of the first line equal to text at or after
// start, or -1.
func indexFrom(lines []string, text string, start int) int {
	for index := start; index < len(lines); index++ {
		if lines[index] == text {
			return index
		}
	}
	return -1
}

func TestADeltaAppliedToItsBaseGivesTheTarget(t *testing.T) {
	cases := []struct{ name, base, target string }{
		{"a line added in the middle", "a\nb\nc\n", "a\nb\nx\nc\n"},
		{"a line removed at the start", "a\nb\nc\n", "b\nc\n"},
		{"a line changed at the end", "a\nb\nc\n", "a\nb\nz\n"},
		{"repeated lines keep their places", "a\nb\na\nb\n", "a\nb\nx\na\nb\ny\n"},
		{"an empty base", "", "x\ny\n"},
		{"an empty target", "x\ny\n", ""},
		{"equal texts", "a\nb\n", "a\nb\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			delta := makeDelta(test.base, test.target)
			got, err := applyDelta(test.base, delta)
			if err != nil || got != test.target {
				t.Errorf("applyDelta(%q, %q) = %q, %v; want %q", test.base, delta, got, err, test.target)
			}
		})
	}
}

func TestDiffLinesFindsTheShortestEditScript(t *testing.T) {
	cases := []struct {
		name         string
		base, target []string
		changes      int
	}{
		{"one line added", []string{"a", "b", "c"}, []string{"a", "x", "b", "c"}, 1},
		{"one line changed", []string{"a", "b", "c"}, []string{"a", "x", "c"}, 2},
		{"the classic example", strings.Split("abcabba", ""), strings.Split("cbabac", ""), 5},
		{"nothing in common", []string{"a", "b"}, []string{"c", "d"}, 4},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			changes := 0
			for _, step := range diffLines(test.base, test.target) {
				if step.kind != ' ' {
					changes++
				}
			}
			if changes != test.changes {
				t.Errorf("diffLines(%q, %q) made %d changes, want %d", test.base, test.target, changes, test.changes)
			}
		})
	}
}

func TestADeltaWhoseRemovedLineIsGoneDoesNotApply(t *testing.T) {
	delta := makeDelta("a\nb\nc\n", "a\nc\n")
	if _, err := applyDelta("a\nx\nc\n", delta); !errors.Is(err, errDeltaDoesNotApply) {
		t.Errorf("applyDelta over a changed base: error %v, want errDeltaDoesNotApply", err)
	}
}
