package glob

import (
	"slices"
	"testing"
)

// matchPathCase is one MatchPath test: a pattern, a path and the result.
type matchPathCase struct {
	name    string
	pattern string
	path    string
	want    bool
}

// matchPathCorpus returns the MatchPath cases every platform shares.
func matchPathCorpus() []matchPathCase {
	return []matchPathCase{
		{"a star crosses a slash", "*/[Zz]ed/node", "AppData/Local/Zed/node", true},
		{"a star crosses a newline", "a*b", "a\nb", true},
		{"a question mark matches a slash", "a?b", "a/b", true},
		{"a question mark matches a newline", "a?b", "a\nb", true},
		{"a leading dot needs no literal dot", "*/auth.json", ".hermes/auth.json", true},
		{"a star matches the empty string", "a*", "a", true},
		{"stars backtrack", "a*b*c", "aXbYbZc", true},
		{"the whole path must match", "a*", "ba", false},
		{"a class matches one of its bytes", "[Cc]rew", "crew", true},
		{"a negated class excludes its bytes", "[!.]*", ".env", false},
		{"a negated class takes other bytes", ".[!e]*", ".x", true},
		{"a range matches inside it", "[0-9a-f]", "c", true},
		{"a range compares bytes unsigned", "[a-\xff]", "\xe9", true},
		{"a reversed range matches nothing", "[z-a]", "m", false},
		{"a ] first in a class is literal", "[]a]", "]", true},
		{"a ] first after ! is literal", "[!]a]", "]", false},
		{"a - last in a class is literal", "[a-]", "-", true},
		{"a caret does not negate", "[^a]", "a", true},
		{"a POSIX class knows no byte above 0x7f", "[[:alpha:]]", "\xe9", false},
		{"an unknown class fails the match", "[[:foo:]a]", "a", false},
		{"an unknown class after the matching item goes unnoticed", "[a[:foo:]]", "a", true},
		{"a class cannot start a range", "[[:alpha:]-z]", "a", false},
		{"a collating symbol is its byte", "[[.-.]]", "-", true},
		{"an equivalence class is its byte", "[[=a=]]", "a", true},
		{"an unclosed bracket never matches", "x[", "x[", false},
		{"a backslash escapes a star", `q\*`, "q*", true},
		{"an escaped star is not a wildcard", `q\*`, "qz", false},
		{"an escaped caret matches a backslash and a caret", `\^`, `\^`, true},
		{"an escaped ] in a class is a member", `[\]1]`, "1", true},
		{"the skip after a match reads a backslash as plain", `[ab\]c]`, "ac]", true},
		{"so the bytes before an escaped ] cannot match alone", `[ab\]c]`, "a", false},
		{"a trailing backslash never matches", `x\`, `x\`, false},
		{"an empty pattern matches the empty string", "", "", true},
	}
}

func TestMatchPath(t *testing.T) {
	for _, testCase := range append(matchPathCorpus(), platformMatchPathCases...) {
		t.Run(testCase.name, func(t *testing.T) {
			if got := MatchPath(testCase.pattern, testCase.path); got != testCase.want {
				t.Errorf("MatchPath(%q, %q) = %v, want %v",
					testCase.pattern, testCase.path, got, testCase.want)
			}
		})
	}
}

func TestMatchSegment(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		segment string
		want    bool
	}{
		{"a star skips a leading dot", "*", ".env", false},
		{"a question mark skips a leading dot", "?env", ".env", false},
		{"a class skips a leading dot", "[.]env", ".env", false},
		{"a negated class skips a leading dot", "[!a]env", ".env", false},
		{"a literal leading dot opens dot names", ".*", ".env", true},
		{"an escaped leading dot opens dot names", `\.*`, ".env", true},
		{"a dot pattern matches the directory itself", ".*", ".", true},
		{"a dot pattern matches the parent", ".*", "..", true},
		{"a dot later in the name needs nothing", "*.json", "a.json", true},
		{"a star matches a newline in a name", "nl*", "nl\n", true},
		{"a segment without a wildcard is literal text", `x\y`, "xy", true},
		{"an unclosed bracket is literal text", "x[", "x[", true},
		{"an empty class is literal text", "[]", "[]", true},
		{"a closed class is a wildcard", "[ab]", "[ab]", false},
		{"names compare bytewise", "[Aa]ntigravity", "antigravity", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := MatchSegment(testCase.pattern, testCase.segment); got != testCase.want {
				t.Errorf("MatchSegment(%q, %q) = %v, want %v",
					testCase.pattern, testCase.segment, got, testCase.want)
			}
		})
	}
}

func TestQuoteMetaMakesAPatternMatchItsTextOnly(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		other string
	}{
		{"a bracket", "/home/a[b]c", "/home/abc"},
		{"a star", "/home/*", "/home/x"},
		{"a question mark", "/home/?", "/home/x"},
		{"a backslash", `/home/a\b`, "/home/ab"},
		{"a caret", "/home/^a", "/home/a"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pattern := QuoteMeta(testCase.text) + "/*"
			if !MatchPath(pattern, testCase.text+"/x") {
				t.Errorf("%q does not match %q", pattern, testCase.text+"/x")
			}
			if MatchPath(pattern, testCase.other+"/x") {
				t.Errorf("%q matches %q", pattern, testCase.other+"/x")
			}
		})
	}
}

// identity is the Unix fold: names compared byte for byte.
func identity(text string) string { return text }

func TestFoldASCIILowersOnlyASCIICapitals(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"capitals are lowered", "AppData/Local", "appdata/local"},
		{"the letters around the range stay", "@[`{", "@[`{"},
		{"bytes above 0x7f stay", "\xc3\x89T\xff", "\xc3\x89t\xff"},
		{"the empty text stays", "", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := foldASCII(testCase.text); got != testCase.want {
				t.Errorf("foldASCII(%q) = %q, want %q", testCase.text, got, testCase.want)
			}
		})
	}
}

func TestSegmentsMatchAListOfNamesUnderEachFold(t *testing.T) {
	names := []string{"Users", "USERS", "users", "Users2", ".Users", "AppData", "appdata",
		"\u0131mg", "img"}
	cases := []struct {
		name     string
		pattern  string
		unixWant []string
		foldWant []string
	}{
		{"a literal segment compares the whole name", "users",
			[]string{"users"}, []string{"Users", "USERS", "users"}},
		{"an escaped literal segment compares unescaped", `U\sers`,
			[]string{"Users"}, []string{"Users", "USERS", "users"}},
		{"a star ignores case only under the fold", "U*",
			[]string{"Users", "USERS", "Users2"}, []string{"Users", "USERS", "users", "Users2"}},
		{"a class folds its bytes as written", "[a]pp*",
			[]string{"appdata"}, []string{"AppData", "appdata"}},
		{"a range folds as written", "[A-Z]PPDATA",
			nil, []string{"AppData", "appdata"}},
		{"the leading-dot rule holds under the fold", "*users",
			[]string{"users"}, []string{"Users", "USERS", "users"}},
		{"a leading dot opens dot names under the fold", ".u*",
			nil, []string{".Users"}},
		{"dotless i stays a different name under the fold, as NTFS keeps it", "IMG",
			nil, []string{"img"}},
		{"a star after a capital I never takes dotless i", "I*",
			nil, []string{"img"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, fold := range []struct {
				name     string
				function func(string) string
				want     []string
			}{{"unix", identity, testCase.unixWant}, {"windows", foldASCII, testCase.foldWant}} {
				var got []string
				for _, name := range names {
					if matchSegment(testCase.pattern, name, fold.function) {
						got = append(got, name)
					}
				}
				if !slices.Equal(got, fold.want) {
					t.Errorf("%s fold: %q matches %q, want %q", fold.name, testCase.pattern, got, fold.want)
				}
			}
		})
	}
}
