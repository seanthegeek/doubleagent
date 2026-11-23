package parserkit

import (
	"slices"
	"strings"
	"testing"
)

// compositeStep sets one piece, then asks whether the text changed and
// records it when it did.
type compositeStep struct {
	piece int
	text  string
}

// joinedChanges returns, for each step, whether the joined pieces differ
// from the joined pieces last recorded, joining them every time.
func joinedChanges(n int, steps []compositeStep) []bool {
	pieces, recorded, held := make([]string, n), "", false
	var changes []bool
	for _, step := range steps {
		pieces[step.piece] = step.text
		joined := strings.Join(pieces, "")
		changed := !held || joined != recorded
		if changed {
			recorded, held = joined, true
		}
		changes = append(changes, changed)
	}
	return changes
}

// A Composite answers as comparing the joined texts does, also where the
// pieces differ but join to the same text.
func TestACompositeChangesWhenItsJoinedTextChanges(t *testing.T) {
	tests := []struct {
		name  string
		steps []compositeStep
	}{
		{"the first text is a change", []compositeStep{{0, "a"}}},
		{"setting a piece to its value is no change", []compositeStep{{0, "a"}, {1, " b"}, {1, " b"}, {0, "a"}}},
		{"a piece of the same length changing is a change", []compositeStep{{0, "a"}, {1, " b"}, {1, " c"}}},
		{"a piece set and set back is no change", []compositeStep{{0, "a"}, {1, " b"}, {1, " c"}, {0, "a"}}},
		{"a piece set back after a record is a change", []compositeStep{{0, "a"}, {1, " b"}, {1, " bb"}, {1, " b"}}},
		{"pieces that split one text another way are no change",
			[]compositeStep{{0, "x agent=y"}, {1, ""}, {0, "x"}, {1, " agent=y"}, {1, " agent=y"}}},
		{"a split that later joins differently is a change",
			[]compositeStep{{0, "x agent=y"}, {0, "x"}, {1, " agent=y"}, {1, " agent=z"}}},
		{"a longer text is a change", []compositeStep{{0, "a"}, {1, " b"}, {0, "aa"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			composite := NewComposite(2, Work{})
			var got []bool
			for _, step := range test.steps {
				composite.Set(step.piece, step.text)
				changed := composite.Changed()
				if changed {
					composite.Record()
				}
				got = append(got, changed)
			}
			if want := joinedChanges(2, test.steps); !slices.Equal(got, want) {
				t.Errorf("changes %v, want %v", got, want)
			}
		})
	}
}

// A long piece no event sets again is never joined or compared, however
// many events change the short one, and a split that joins to the recorded
// text is compared once until a piece changes.
func TestACompositeReadsOnlyThePiecesAnEventSets(t *testing.T) {
	work := CountingWork()
	composite := NewComposite(2, work)
	composite.Set(0, strings.Repeat("M", 100_000))
	for i := range 1000 {
		composite.Set(1, " agent="+string(rune('a'+i%2)))
		if !composite.Changed() {
			t.Fatalf("event %d changed nothing", i)
		}
		composite.Record()
	}
	if compared := work.Count("composite bytes compared"); compared != 0 {
		t.Errorf("%d bytes of joined text compared, want none", compared)
	}
	composite.Set(0, "x agent=y")
	composite.Set(1, "")
	composite.Record()
	composite.Set(0, "x")
	composite.Set(1, " agent=y")
	for range 10 {
		if composite.Changed() {
			t.Fatal("a split joining to the recorded text is a change")
		}
	}
	if compared := work.Count("composite bytes compared"); compared != 2*len("x agent=y") {
		t.Errorf("%d bytes of joined text compared, want one comparison of %d", compared, len("x agent=y"))
	}
}
