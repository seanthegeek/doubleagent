package parserkit

import (
	"fmt"
	"regexp"
	"regexp/syntax"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// RelPattern is a regular expression over a path relative to a home that
// ignores the case of ASCII letters, as the catalog and collect match
// paths: rel matches when the expression, as written, matches rel with
// some choice of case for each of its ASCII letters. No other letter
// folds, so neither KELVIN SIGN nor LONG S stands for k or s, and a
// negated class leaves out a letter only when it leaves out both cases
// ([^A-Za-z] matches no letter, [^A-Z] every one). Each value it returns
// is cut from rel as given, keeping the spelling the disk gave.
type RelPattern struct {
	lowered *regexp.Regexp
}

// caseFlag finds a flag group that sets the i flag, whose Unicode folding
// would let a letter outside ASCII stand for an ASCII one.
var caseFlag = regexp.MustCompile(`\(\?[imsU-]*i[imsU-]*[:)]`)

// MustRelPattern returns the RelPattern of expression, written as the agent
// spells the path in the syntax regexp.MustCompile takes. Like that
// function it panics when the expression does not compile, so a pattern
// is a package value checked when the package loads; it also panics on an
// expression that sets the i flag, which ASCII matching makes needless.
func MustRelPattern(expression string) *RelPattern {
	if caseFlag.MatchString(expression) {
		panic(fmt.Sprintf("parserkit: MustRelPattern(%q): the i flag folds letters outside ASCII", expression))
	}
	tree, err := syntax.Parse(expression, syntax.Perl)
	if err != nil {
		panic(fmt.Sprintf("parserkit: MustRelPattern(%q): %v", expression, err))
	}
	lowerTree(tree)
	return &RelPattern{lowered: regexp.MustCompile(tree.String())}
}

// lowerTree rewrites a parsed expression in place to match LowerASCII of
// each rel the expression matches in any ASCII case: every capital of a
// literal is lowered, and every class also holds the small letter of each
// capital it holds. A literal the parser made from a pair such as [Aa]
// carries the fold flag; it is dropped when every rune is ASCII, and kept
// otherwise, where the parser set it only for a rune and its one other case.
func lowerTree(tree *syntax.Regexp) {
	switch tree.Op {
	case syntax.OpLiteral:
		ascii := true
		for i, char := range tree.Rune {
			if 'A' <= char && char <= 'Z' {
				tree.Rune[i] = char + ('a' - 'A')
			}
			ascii = ascii && char < 0x80
		}
		if ascii {
			tree.Flags &^= syntax.FoldCase
		}
	case syntax.OpCharClass:
		tree.Rune = lowerClass(tree.Rune)
	}
	for _, sub := range tree.Sub {
		lowerTree(sub)
	}
}

// lowerClass returns a class's ranges with the small letters of the
// capitals they hold added after them; the parse of the printed class
// sorts and merges them.
func lowerClass(ranges []rune) []rune {
	extended := ranges
	for i := 0; i+1 < len(ranges); i += 2 {
		low, high := max(ranges[i], 'A'), min(ranges[i+1], 'Z')
		if low <= high {
			extended = append(extended, low+('a'-'A'), high+('a'-'A'))
		}
	}
	return extended
}

// MatchString reports whether rel matches the pattern.
func (p *RelPattern) MatchString(rel string) bool {
	return p.lowered.MatchString(textlines.LowerASCII(rel))
}

// FindStringIndex returns the start and end of the leftmost match in rel,
// or nil when there is none.
func (p *RelPattern) FindStringIndex(rel string) []int {
	return p.lowered.FindStringIndex(textlines.LowerASCII(rel))
}

// FindStringSubmatch returns the leftmost match in rel and its groups, each
// cut from rel as given, "" for a group that took no part, or nil when
// there is no match.
func (p *RelPattern) FindStringSubmatch(rel string) []string {
	places := p.lowered.FindStringSubmatchIndex(textlines.LowerASCII(rel))
	if places == nil {
		return nil
	}
	found := make([]string, len(places)/2)
	for i := range found {
		if places[2*i] >= 0 {
			found[i] = rel[places[2*i]:places[2*i+1]]
		}
	}
	return found
}

// SubexpIndex returns the index of the group with the given name, or -1.
func (p *RelPattern) SubexpIndex(name string) int {
	return p.lowered.SubexpIndex(name)
}

// SubexpNames returns the names of the groups, "" for an unnamed one, the
// first for the whole match.
func (p *RelPattern) SubexpNames() []string {
	return p.lowered.SubexpNames()
}
