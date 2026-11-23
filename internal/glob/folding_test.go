package glob

import (
	"slices"
	"strings"
	"testing"
)

// componentsFold returns a fold test for path that answers true in the
// components numbered in folding, counted from 0 at the start of path.
func componentsFold(path string, folding ...int) func(int) bool {
	return func(index int) bool {
		component := strings.Count(path[:index], "/")
		return slices.Contains(folding, component)
	}
}

// never is the fold test that folds no byte.
func never(int) bool { return false }

// always is the fold test that folds every byte.
func always(int) bool { return true }

func TestFoldingMasksAtTheirExtremesEqualTheByteAndTheFoldedMatch(t *testing.T) {
	corpus := append(matchPathCorpus(),
		matchPathCase{"a class is case-sensitive", "[a]", "A", false},
		matchPathCase{"a POSIX class matches", "[[:upper:]]", "Q", true},
		matchPathCase{"a range of capitals", "[A-Z]oose", "goose", false},
		matchPathCase{"a negated letter", "[!a]", "A", true},
		matchPathCase{"a literal capital", ".config/Goose/secrets.yaml", ".config/goose/secrets.yaml", false},
	)
	for _, test := range corpus {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			byBytes := match(test.pattern, test.path)
			folded := match(foldASCII(test.pattern), foldASCII(test.path))
			if got := MatchPathFolding(test.pattern, test.path, never); got != byBytes {
				t.Errorf("no folding gives %v, the byte match %v", got, byBytes)
			}
			if got := MatchPathFolding(test.pattern, test.path, always); got != folded {
				t.Errorf("folding everywhere gives %v, the folded match %v", got, folded)
			}
			pattern := NewFoldingPattern(test.pattern)
			if pattern.MatchBytes(test.path) != byBytes {
				t.Errorf("MatchBytes differs from the byte match %v", byBytes)
			}
			if (byBytes || folded) && !pattern.MatchEitherCase(test.path) {
				t.Errorf("MatchEitherCase misses a match one extreme finds")
			}
		})
	}
}

func TestFoldingAppliesOnlyWhereTheMaskSaysSo(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		path    string
		folding []int
		want    bool
	}{
		{"a folding directory takes another spelling", ".config/goose/secrets.yaml",
			".config/Goose/secrets.yaml", []int{1}, true},
		{"a directory that does not fold keeps case", ".config/goose/secrets.yaml",
			".config/Goose/secrets.yaml", []int{0, 2}, false},
		{"a fold of the wrong component does not help", ".config/goose/secrets.yaml",
			".config/Goose/Secrets.yaml", []int{1}, false},
		{"a range of capitals is lowered where it folds", "a/[A-Z]", "a/q", []int{1}, true},
		{"a range of capitals keeps case where it does not", "a/[A-Z]", "a/q", []int{0}, false},
		{"a negated letter takes the other case by byte", "x/[!a]", "x/A", nil, true},
		{"a negated letter refuses the other case where it folds", "x/[!a]", "x/A", []int{1}, false},
		{"one path can need a byte compare and a fold", "[!a]/B", "A/b", []int{1}, true},
		{"a star crossing into a folding component", "*/Cache", "Goose/cache", []int{1}, true},
		{"a star crossing into a component that does not fold", "*/Cache", "Goose/cache",
			[]int{0}, false},
		{"a star crossing a folding component reaches a literal one", "*oose/Cache",
			"GOOSE/Cache", []int{0}, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			folds := componentsFold(test.path, test.folding...)
			if got := MatchPathFolding(test.pattern, test.path, folds); got != test.want {
				t.Errorf("MatchPathFolding(%q, %q, components %v fold) = %v, want %v",
					test.pattern, test.path, test.folding, got, test.want)
			}
			if test.want && !NewFoldingPattern(test.pattern).MatchEitherCase(test.path) {
				t.Errorf("MatchEitherCase misses a match the components %v find", test.folding)
			}
		})
	}
}

func TestMatchEitherCaseBoundsEveryMask(t *testing.T) {
	patterns := []string{"[!a]/B", "*/[!x]oose/*", "a/[A-Z]", ".config/goose/*", "*/Cache"}
	paths := []string{"A/b", "a/B", "x/Xoose/c", "a/q", ".config/Goose/f", "Goose/cache"}
	for _, text := range patterns {
		pattern := NewFoldingPattern(text)
		for _, path := range paths {
			either := pattern.MatchEitherCase(path)
			components := strings.Count(path, "/") + 1
			for choice := range 1 << components {
				var folding []int
				for component := range components {
					if choice&(1<<component) != 0 {
						folding = append(folding, component)
					}
				}
				folds := componentsFold(path, folding...)
				if pattern.Match(path, folds) && !either {
					t.Errorf("%q matches %q under fold choice %b; MatchEitherCase says no",
						text, path, choice)
				}
			}
		}
	}
}
