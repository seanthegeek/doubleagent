package collect_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A delta is a file stored as its differences from a base file: the default
// case's file of the same name, or unix/'s for a platform's own view. Each
// hunk starts with "@@ at start", or with "@@ after" and the base lines it
// follows, each after a space, as few as place it (the first place after
// the previous hunk where those lines stand), then the base lines it
// removes, each after "-", and the lines it adds, each after "+". A change
// elsewhere in the base leaves the delta as it was.

// deltaHeader opens every delta file, to say what it is to a reader.
const deltaHeader = "# the base file with these changes: each hunk follows the lines it quotes\n"

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
	lines := splitLines(base)
	var out strings.Builder
	out.WriteString(deltaHeader)
	position, next, inHunk := 0, 0, false
	for _, step := range diffLines(lines, splitLines(target)) {
		if step.kind == ' ' {
			next, inHunk = next+1, false
			continue
		}
		if !inHunk {
			out.WriteString(hunkStart(lines, position, next-1))
			inHunk = true
		}
		out.WriteString(string(step.kind) + step.line + "\n")
		if step.kind == '-' {
			next++
		}
		position = next
	}
	return out.String()
}

// hunkStart returns the start of a hunk that follows base line anchor (-1
// for none), after the previous hunk ended at base line position: the
// fewest lines ending at anchor whose first place from position is anchor.
func hunkStart(lines []string, position, anchor int) string {
	if anchor < position {
		return "@@ at start\n"
	}
	count := 1
	for contextEnd(lines, lines[anchor-count+1:anchor+1], position) != anchor {
		count++
	}
	var out strings.Builder
	out.WriteString("@@ after\n")
	for _, line := range lines[anchor-count+1 : anchor+1] {
		out.WriteString(" " + line + "\n")
	}
	return out.String()
}

// contextEnd returns the index of the last line of the first place at or
// after start where lines holds context, or -1.
func contextEnd(lines, context []string, start int) int {
	for end := start + len(context) - 1; end < len(lines); end++ {
		if slices.Equal(lines[end-len(context)+1:end+1], context) {
			return end
		}
	}
	return -1
}

// errDeltaDoesNotApply is the error of a delta whose quoted or removed lines
// the base does not hold where the delta says.
var errDeltaDoesNotApply = errors.New("the delta does not apply to its base file")

// applyDelta returns base with the delta's changes.
func applyDelta(base, delta string) (string, error) {
	lines := splitLines(base)
	steps := splitLines(strings.TrimPrefix(delta, deltaHeader))
	var out []string
	position := 0
	for index := 0; index < len(steps); index++ {
		line := steps[index]
		switch {
		case line == "@@ at start":
		case line == "@@ after":
			var context []string
			for index+1 < len(steps) && strings.HasPrefix(steps[index+1], " ") {
				index++
				context = append(context, steps[index][1:])
			}
			end := contextEnd(lines, context, position)
			if len(context) == 0 || end < 0 {
				return "", fmt.Errorf("line %d: quoted lines not found: %w", index+2, errDeltaDoesNotApply)
			}
			out, position = append(out, lines[position:end+1]...), end+1
		case strings.HasPrefix(line, "-"):
			if position >= len(lines) || lines[position] != line[1:] {
				return "", fmt.Errorf("line %d: removed line not found: %w", index+2, errDeltaDoesNotApply)
			}
			position++
		case strings.HasPrefix(line, "+"):
			out = append(out, line[1:])
		default:
			return "", fmt.Errorf("line %d: %q is no delta line: %w", index+2, line, errDeltaDoesNotApply)
		}
	}
	return joinLines(append(out, lines[position:]...)), nil
}

func TestADeltaAppliedToItsBaseGivesTheTarget(t *testing.T) {
	cases := []struct{ name, base, target string }{
		{"a line added in the middle", "a\nb\nc\n", "a\nb\nx\nc\n"},
		{"a line removed at the start", "a\nb\nc\n", "b\nc\n"},
		{"a line changed at the end", "a\nb\nc\n", "a\nb\nz\n"},
		{"repeated lines keep their places", "a\nb\na\nb\n", "a\nb\nx\na\nb\ny\n"},
		{"a change after repeated lines", "{\n x\n}\n{\n x\n}\n", "{\n x\n}\n{\n y\n}\n"},
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
