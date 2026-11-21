package dedup

import (
	"strings"
	"testing"
)

// piecesText returns a match text of pieces, an empty one included as it
// is: a piece starting with "#" is a segment whose writer yields the parts
// of the rest between "|" separators, empty parts too, and adds one to
// *writes each time it runs; any other piece is literal.
func piecesText(texts *Texts, writes *int, pieces ...string) *MatchText {
	text := texts.NewText()
	write := func(full string, _ uint64, yield func(string) bool) uint64 {
		*writes++
		for chunk := range strings.SplitSeq(full, "|") {
			if !yield(chunk) {
				break
			}
		}
		return 0
	}
	for _, piece := range pieces {
		full, isSegment := strings.CutPrefix(piece, "#")
		if !isSegment {
			text.parts = append(text.parts, textPart{literal: piece})
			continue
		}
		text.parts = append(text.parts, textPart{segment: &segment{}, full: full, write: write})
		text.segmented = true
	}
	return text
}

func TestTwoTextsInPiecesAreEqualExactlyWhenTheirBytesAre(t *testing.T) {
	tests := []struct {
		name        string
		left, right []string
		equal       bool
	}{
		{"equal texts cut at other boundaries are equal", []string{"ab", "cd"}, []string{"a", "bcd"}, true},
		{"equal texts cut inside segments are equal", []string{"#ab|c", "d"}, []string{"a", "#bc|d"}, true},
		{"equal texts in segments alone are equal", []string{"#a|bc", "#d"}, []string{"#ab", "#c|d"}, true},
		{"texts differing in the first piece differ", []string{"xb", "cd"}, []string{"ab", "cd"}, false},
		{"texts differing in a first segment differ", []string{"#x|b", "cd"}, []string{"ab", "#cd"}, false},
		{"texts differing in the last piece differ", []string{"ab", "cx"}, []string{"a", "bcd"}, false},
		{"texts differing in a last segment differ", []string{"ab", "#c|x"}, []string{"#a|b", "#cd"}, false},
		{"a text and a longer one it begins differ", []string{"ab"}, []string{"ab", "c"}, false},
		{"a segment and a longer one it begins differ", []string{"#a|b"}, []string{"#a|b|c"}, false},
		{"empty pieces in the middle are skipped", []string{"ab", "", "#|c||", "#", "d"}, []string{"abcd"}, true},
		{"two empty texts are equal", nil, nil, true},
		{"empty pieces and an empty text are equal", []string{"", "#|"}, nil, true},
		{"an empty text and one that is not differ", nil, []string{"a"}, false},
		{"empty pieces and a text that is not empty differ", []string{"", "#|"}, []string{"#|a"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			texts, writes := NewTexts(), 0
			left, right := piecesText(texts, &writes, test.left...), piecesText(texts, &writes, test.right...)
			if got := sameText(left, right); got != test.equal {
				t.Errorf("%q and %q equal: %t, want %t", test.left, test.right, got, test.equal)
			}
			if got := sameText(right, left); got != test.equal {
				t.Errorf("%q and %q equal: %t, want %t", test.right, test.left, got, test.equal)
			}
		})
	}
}

func TestTwoTextsAreReadOnlyAsFarAsTheirFirstDifference(t *testing.T) {
	tests := []struct {
		name        string
		left, right []string
	}{
		{"literal texts differing first", []string{"x", "#ab"}, []string{"y", "#ab"}},
		{"a segmented text and a literal one differing first", []string{"x", "#ab"}, []string{"y", "ab"}},
		{"a literal text and a segmented one differing first", []string{"y", "ab"}, []string{"x", "#ab"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			texts, writes := NewTexts(), 0
			left, right := piecesText(texts, &writes, test.left...), piecesText(texts, &writes, test.right...)
			if sameText(left, right) || writes != 0 || texts.SharedRead() != 0 {
				t.Errorf("%q and %q: segments written %d times, %d shared bytes read, want none", test.left,
					test.right, writes, texts.SharedRead())
			}
		})
	}
}

// The comparison allocates its cursor, which the function handed to a
// writer captures, and for each segment written as the cursor reads it the
// function handed to the writer, the flag it sets and the cursor's method
// value: a fixed count per comparison and per segment, never per piece.
func TestComparingTwoTextsAllocatesOnlyTheCursorAndThreeTimesPerSegment(t *testing.T) {
	texts, writes := NewTexts(), 0
	plain := piecesText(texts, &writes, "ab", "", "cd", "e")
	tests := []struct {
		name        string
		other       *MatchText
		allocations float64
	}{
		{"two literal texts allocate the cursor alone", piecesText(texts, &writes, "a", "bcd", "e"), 1},
		{"a literal text and one with a segment allocate four times",
			piecesText(texts, &writes, "a", "#bc|d", "e"), 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, pair := range [][2]*MatchText{{plain, test.other}, {test.other, plain}} {
				allocations := testing.AllocsPerRun(100, func() {
					if !sameText(pair[0], pair[1]) {
						t.Fatal("equal texts compared unequal")
					}
				})
				if allocations > test.allocations {
					t.Errorf("comparing %q with %q allocated %v times, want at most %v", pair[0], pair[1],
						allocations, test.allocations)
				}
			}
		})
	}
}
