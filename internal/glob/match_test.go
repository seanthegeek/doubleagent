package glob

import "testing"

func TestMatchPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"a star crosses a slash", "*/zed/node", "AppData/Local/Zed/node", true},
		{"a star crosses a newline", "a*b", "a\nb", true},
		{"a question mark matches a slash", "a?b", "a/b", true},
		{"a leading dot needs no literal dot", "*/auth.json", ".hermes/auth.json", true},
		{"a star matches the empty string", "a*", "a", true},
		{"stars backtrack", "a*b*c", "aXbYbZc", true},
		{"the whole path must match", "a*", "ba", false},
		{"a literal ignores ASCII case", "AppData/Local", "appdata/LOCAL", true},
		{"a byte above 0x7f is compared as it is", "\xc3\x89t\xff", "\xc3\xa9t\xff", false},
		{"dotless i is not I", "img", "ımg", false},
		{"a list matches one of its bytes in either case", "[c]rew", "Crew", true},
		{"a negated list excludes its bytes", "[!.]*", ".env", false},
		{"a negated list ignores case", "[!a]", "A", false},
		{"a negated list takes other bytes", ".[!e]*", ".x", true},
		{"a range matches inside it", "[0-9a-f]", "c", true},
		{"a range is lowered as written", "[A-Z]", "q", true},
		{"a range compares bytes unsigned", "[a-\xff]", "\xe9", true},
		{"a reversed range matches nothing", "[z-a]", "m", false},
		{"a ] first in a list is a member", "[]a]", "]", true},
		{"a ] first after ! is a member", "[!]a]", "]", false},
		{"a - last in a list is a member", "[a-]", "-", true},
		{"a caret is a plain member", "[^a]", "^", true},
		{"a class name matches nothing", "[[:alpha:]]", "a", false},
		{"an unclosed bracket matches nothing", "x[", "x[", false},
		{"an empty list matches nothing", "[]", "[]", false},
		{"a backslash is a plain byte", `q\*`, `q\z`, true},
		{"an empty pattern matches the empty string", "", "", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := MatchPath(testCase.pattern, testCase.path); got != testCase.want {
				t.Errorf("MatchPath(%q, %q) = %v, want %v",
					testCase.pattern, testCase.path, got, testCase.want)
			}
		})
	}
}

func TestMatchSegment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		pattern string
		segment string
		want    bool
	}{
		{"a star skips a leading dot", "*", ".env", false},
		{"a question mark skips a leading dot", "?env", ".env", false},
		{"a list skips a leading dot", "[.]env", ".env", false},
		{"a negated list skips a leading dot", "[!a]env", ".env", false},
		{"a literal leading dot opens dot names", ".*", ".env", true},
		{"a dot later in the name needs nothing", "*.json", "a.json", true},
		{"a star matches a newline in a name", "nl*", "nl\n", true},
		{"a literal segment ignores case", "Goose", "goose", true},
		{"a list ignores case", "[a]ntigravity", "Antigravity", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := MatchSegment(testCase.pattern, testCase.segment); got != testCase.want {
				t.Errorf("MatchSegment(%q, %q) = %v, want %v",
					testCase.pattern, testCase.segment, got, testCase.want)
			}
		})
	}
}
