package glob

import (
	"slices"
	"testing"
)

// matchTable is one pattern and every subject, of the curated subjects or the
// newline subjects, it matches.
type matchTable struct {
	pattern string
	matches []string
}

// curatedMatches holds, for each curated pattern, the curated subjects it
// matches, as POSIX sh's case statement decides them with LC_ALL=C.
var curatedMatches = []matchTable{
	{"*", []string{"", "a", "b", "c", "z", "-", "]", "[", "^", "\\", "\\^", "*", "?", ".", "x", "x\\", "ab", "abc", "aXbYc", "a/x", "a/b/x", ".hidden", ".h/auth.json", ".env", ".e", "0f", "x/Zed/node", "y/zed/node", "a/zed/node/z", "a.json", "a/b.json", "[a]", "x[", "[a", "!", "1"}},
	{"?", []string{"a", "b", "c", "z", "-", "]", "[", "^", "\\", "*", "?", ".", "x", "!", "1"}},
	{"a*", []string{"a", "ab", "abc", "aXbYc", "a/x", "a/b/x", "a/zed/node/z", "a.json", "a/b.json"}},
	{"*a", []string{"a", "[a"}},
	{"a?c", []string{"abc"}},
	{"a*b*c", []string{"abc", "aXbYc"}},
	{"[ab]", []string{"a", "b"}},
	{"[!ab]", []string{"c", "z", "-", "]", "[", "^", "\\", "*", "?", ".", "x", "!", "1"}},
	{"[a-c]", []string{"a", "b", "c"}},
	{"[!a-c]", []string{"z", "-", "]", "[", "^", "\\", "*", "?", ".", "x", "!", "1"}},
	{"[]a]", []string{"a", "]"}},
	{"[!]a]", []string{"b", "c", "z", "-", "[", "^", "\\", "*", "?", ".", "x", "!", "1"}},
	{"[a-]", []string{"a", "-"}},
	{"[]-a]", []string{"a", "]", "^"}},
	{"[z-a]", []string{}},
	{"[[:alpha:]]", []string{"a", "b", "c", "z", "x"}},
	{"[![:digit:]]", []string{"a", "b", "c", "z", "-", "]", "[", "^", "\\", "*", "?", ".", "x", "!"}},
	{"[]", []string{}},
	{"[!]", []string{}},
	{"*/x", []string{"a/x", "a/b/x"}},
	{"*.json", []string{".h/auth.json", "a.json", "a/b.json"}},
	{".h*/auth.json", []string{".h/auth.json"}},
	{"[!.]*", []string{"a", "b", "c", "z", "-", "]", "[", "^", "\\", "\\^", "*", "?", "x", "x\\", "ab", "abc", "aXbYc", "a/x", "a/b/x", "0f", "x/Zed/node", "y/zed/node", "a/zed/node/z", "a.json", "a/b.json", "[a]", "x[", "[a", "!", "1"}},
	{".[!e]*", []string{".hidden", ".h/auth.json"}},
	{"[0-9a-f][0-9a-f]", []string{"ab", "0f"}},
	{"a/*", []string{"a/x", "a/b/x", "a/zed/node/z", "a/b.json"}},
	{"*/[Zz]ed/node", []string{"x/Zed/node", "y/zed/node"}},
	{"[.]*", []string{".", ".hidden", ".h/auth.json", ".env", ".e"}},
	{"", []string{""}},
}

// newlineMatches holds the subjects with newlines that each pattern matches.
var newlineMatches = []matchTable{
	{"*", []string{"a\nb", "a\n", "\n", "a/\nb", "ab\n"}},
	{"a*", []string{"a\nb", "a\n", "a/\nb", "ab\n"}},
	{"*b", []string{"a\nb", "a/\nb"}},
	{"a?b", []string{"a\nb"}},
	{"a[!x]b", []string{"a\nb"}},
	{"a[[:space:]]b", []string{"a\nb"}},
	{"*\n", []string{"a\n", "\n", "ab\n"}},
	{"a\n*", []string{"a\nb", "a\n"}},
}

func TestMatchPathOfCuratedPatternsAndSubjects(t *testing.T) {
	t.Parallel()
	patterns := curatedPatterns()
	if len(patterns) != len(curatedMatches) {
		t.Fatalf("%d curated patterns but %d expected rows", len(patterns), len(curatedMatches))
	}
	for index, row := range curatedMatches {
		if row.pattern != patterns[index] {
			t.Fatalf("row %d is for %q, want %q", index, row.pattern, patterns[index])
		}
		for _, subject := range curatedSubjects() {
			want := slices.Contains(row.matches, subject)
			if got := MatchPath(row.pattern, subject); got != want {
				t.Errorf("MatchPath(%q, %q) = %v, want %v", row.pattern, subject, got, want)
			}
		}
	}
}

func TestMatchPathOfSubjectsWithNewlines(t *testing.T) {
	t.Parallel()
	subjects := []string{"a\nb", "a\n", "\n", "a/\nb", "ab\n"}
	for _, row := range newlineMatches {
		for _, subject := range subjects {
			want := slices.Contains(row.matches, subject)
			if got := MatchPath(row.pattern, subject); got != want {
				t.Errorf("MatchPath(%q, %q) = %v, want %v", row.pattern, subject, got, want)
			}
		}
	}
}
