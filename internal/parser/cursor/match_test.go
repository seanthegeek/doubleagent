package cursor

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
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
			set := newTextSet(texts, parser.Work{})
			for range 20 {
				text := randomText(source, 30)
				if got, want := set.find(text), containedIn(texts, text); !slices.Equal(got, want) {
					t.Fatalf("texts %q in %q: got %v, want %v", texts, text, got, want)
				}
			}
		})
	}
}

// searchWork returns the work of finding n distinct named texts, as the
// deleted composers' ids, in n texts that each name one of them.
func searchWork(n int) (bytes, fallbacks int) {
	idents := make([]string, n)
	for i := range idents {
		idents[i] = fmt.Sprintf("composer-%08d", i)
	}
	work := parser.CountingWork()
	set := newTextSet(idents, work)
	built := work.Count("fallbacks taken")
	for i := range n {
		set.find(`{"text":"see ` + idents[(i*7)%n] + ` and composer-0000"}`)
	}
	return work.Count("bytes searched"), work.Count("fallbacks taken") - built
}

func TestFindingDeletedComposersInBlobsGrowsWithTheBlobsNotTheirProduct(t *testing.T) {
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
