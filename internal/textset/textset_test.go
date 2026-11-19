package textset

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// randomText returns a text of up to most bytes from a three-letter
// alphabet, so texts overlap, repeat and contain one another often.
func randomText(source *rand.Rand, most int) string {
	var text strings.Builder
	for range source.IntN(most + 1) {
		text.WriteByte("abc"[source.IntN(3)])
	}
	return text.String()
}

// containedIn returns the positions of the texts that text contains,
// ascending, by searching for each in turn.
func containedIn(texts []string, text string) []int {
	var found []int
	for position, each := range texts {
		if strings.Contains(text, each) {
			found = append(found, position)
		}
	}
	return found
}

func TestTheTextSetFindsWhatSearchingForEachTextFinds(t *testing.T) {
	for seed := range uint64(200) {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			texts := make([]string, source.IntN(12))
			for i := range texts {
				texts[i] = randomText(source, 5)
			}
			set := New(texts, nil)
			for range 20 {
				text := randomText(source, 30)
				if got, want := set.Find(text), containedIn(texts, text); !slices.Equal(got, want) {
					t.Fatalf("texts %q in %q: got %v, want %v", texts, text, got, want)
				}
			}
		})
	}
}

// counts is a Counter that keeps its counts by name.
type counts map[string]int

// Add counts n units of the named work.
func (c counts) Add(name string, n int) { c[name] += n }

// searchWork returns the work of finding n distinct named texts, such as
// deleted composers' ids, in n texts that each name one of them.
func searchWork(n int) (bytes, fallbacks int) {
	idents := make([]string, n)
	for i := range idents {
		idents[i] = fmt.Sprintf("composer-%08d", i)
	}
	work := counts{}
	set := New(idents, work)
	built := work["fallbacks taken"]
	for i := range n {
		set.Find(`{"text":"see ` + idents[(i*7)%n] + ` and composer-0000"}`)
	}
	return work["bytes searched"], work["fallbacks taken"] - built
}

func TestFindingTextsGrowsWithTheSearchedTextsNotTheirProduct(t *testing.T) {
	for _, n := range []int{100, 1000} {
		t.Run(fmt.Sprintf("%d composers and blobs", n), func(t *testing.T) {
			bytes, fallbacks := searchWork(n)
			// Each blob is 50 bytes; a search reads each once and falls
			// back at most once per byte read.
			if bytes != 50*n || fallbacks == 0 || fallbacks > bytes {
				t.Errorf("read %d bytes and fell back %d times, want %d bytes and at most as many fallbacks",
					bytes, fallbacks, 50*n)
			}
		})
	}
}

// longestAt returns, for each byte of text, the length of the longest of
// texts that starts there (or, with ending, ends there), by comparing each.
func longestAt(texts []string, text string, ending bool) []int {
	longest := make([]int, len(text))
	for i := range len(text) {
		for _, each := range texts {
			fits := strings.HasPrefix(text[i:], each)
			if ending {
				fits = strings.HasSuffix(text[:i+1], each)
			}
			if fits && len(each) > longest[i] {
				longest[i] = len(each)
			}
		}
	}
	return longest
}

func TestLongestGivesTheLongestTextAtEachPositionAsComparingEachDoes(t *testing.T) {
	for seed := range uint64(200) {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 911))
			texts := make([]string, source.IntN(12))
			for i := range texts {
				texts[i] = randomText(source, 5)
			}
			forward, backward := New(texts, nil), NewBackward(texts, nil)
			for range 20 {
				text := randomText(source, 30)
				for _, test := range []struct {
					set    *Set
					ending bool
				}{{forward, true}, {backward, false}} {
					got := make([]int, len(text))
					test.set.Longest(text, got)
					if want := longestAt(texts, text, test.ending); !slices.Equal(got, want) {
						t.Fatalf("texts %q in %q (ending %v): got %v, want %v", texts, text, test.ending, got, want)
					}
				}
			}
		})
	}
}

func TestABackwardSetFindsWhatAForwardSetFinds(t *testing.T) {
	texts := []string{"abc", "ca", "b", ""}
	for _, text := range []string{"", "xabcx", "cab", "zzz"} {
		if got, want := NewBackward(texts, nil).Find(text), New(texts, nil).Find(text); !slices.Equal(got, want) {
			t.Errorf("%q: backward %v, forward %v", text, got, want)
		}
	}
}

// longestWork returns the bytes read and fallbacks taken finding the
// longest of n texts at each position of a text of n texts joined.
func longestWork(n int) (bytes, fallbacks int) {
	texts := make([]string, n)
	for i := range texts {
		texts[i] = fmt.Sprintf("secret-%06d", i)
	}
	work := counts{}
	set := NewBackward(texts, work)
	built := work["fallbacks taken"]
	text := strings.Join(texts, " secret-")
	set.Longest(text, make([]int, len(text)))
	return work["bytes searched"], work["fallbacks taken"] - built
}

func TestLongestReadsEachByteOnceWhateverTheNumberOfTexts(t *testing.T) {
	for _, n := range []int{100, 1000} {
		t.Run(fmt.Sprintf("%d texts", n), func(t *testing.T) {
			bytes, fallbacks := longestWork(n)
			// Each text is 13 bytes and the joins 8, but the last.
			if want := 21*n - 8; bytes != want || fallbacks > bytes {
				t.Errorf("read %d bytes and fell back %d times, want %d bytes and at most as many fallbacks",
					bytes, fallbacks, want)
			}
		})
	}
}
