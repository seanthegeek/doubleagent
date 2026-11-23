package walk

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// caseFolding is empty on Windows, where glob.MatchPath already ignores
// ASCII case in every component.
type caseFolding struct{}

// newCaseFolding returns the empty value: nothing is probed on Windows.
func newCaseFolding(glob.Check, *fsmeta.Opener) *caseFolding { return &caseFolding{} }

// matchUnder reports whether path[from:] matches one of patterns, ignoring
// ASCII case in every byte as glob.MatchPath does on Windows.
func (*caseFolding) matchUnder(patterns []pathPattern, path string, from, _ int) bool {
	subject := path[from:]
	return slices.ContainsFunc(patterns, func(pattern pathPattern) bool {
		return glob.MatchPath(pattern.text, subject)
	})
}
