package glob

import "testing"

func TestMatchPath(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"a star crosses a slash", "*/[Zz]ed/node", "AppData/Local/Zed/node", true},
		{"a star crosses a newline", "a*b", "a\nb", true},
		{"a question mark matches a slash", "a?b", "a/b", true},
		{"a question mark matches a newline", "a?b", "a\nb", true},
		{"a leading dot needs no literal dot", "*/auth.json", ".hermes/auth.json", true},
		{"a star matches the empty string", "a*", "a", true},
		{"stars backtrack", "a*b*c", "aXbYbZc", true},
		{"the whole path must match", "a*", "ba", false},
		{"a class matches one of its bytes", "[Cc]rew", "crew", true},
		{"a class is case-sensitive", "[a]", "A", false},
		{"a negated class excludes its bytes", "[!.]*", ".env", false},
		{"a negated class takes other bytes", ".[!e]*", ".x", true},
		{"a range matches inside it", "[0-9a-f]", "c", true},
		{"a range compares bytes unsigned", "[a-\xff]", "\xe9", true},
		{"a reversed range matches nothing", "[z-a]", "m", false},
		{"a ] first in a class is literal", "[]a]", "]", true},
		{"a ] first after ! is literal", "[!]a]", "]", false},
		{"a - last in a class is literal", "[a-]", "-", true},
		{"a caret does not negate", "[^a]", "a", true},
		{"a POSIX class matches", "[[:upper:]]", "Q", true},
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
	for _, testCase := range cases {
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
