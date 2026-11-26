package glob

import (
	"os"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/follow"
)

func TestNamesListsADirectoryOnceForTheExpandersLife(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(base+"/Goose", 0o755); err != nil {
		t.Fatal(err)
	}
	expander := NewExpander(nil, nil)
	first, complete := expander.Names(base)
	if err := os.Remove(base + "/Goose"); err != nil {
		t.Fatal(err)
	}
	second, _ := expander.Names(base)
	if want := []string{"Goose"}; !slices.Equal(first, want) || !complete || !slices.Equal(second, want) {
		t.Errorf("Names = %q (complete %t), then %q; want %q twice from one listing",
			first, complete, second, want)
	}
}

func TestNamesListsNothingTheCheckRefuses(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(base+"/Goose", 0o755); err != nil {
		t.Fatal(err)
	}
	refusal := follow.Refusal{Link: base, Kind: follow.LeadsOut}
	expander := NewExpander(func(string) (follow.Refusal, bool) { return refusal, true }, nil)
	// Read whole, so a caller looks nothing up in the directory either.
	if names, complete := expander.Names(base); names != nil || !complete {
		t.Errorf("Names = %q, complete %t; want no names and the listing taken as whole", names, complete)
	}
}
