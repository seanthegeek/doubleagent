package catalog

import (
	"regexp"
	"testing"
)

// syntheticCatalog is the small catalog of the project and claim tests,
// with one home entry whose segment is a class.
const syntheticCatalog = `# home artifacts (agent|glob relative to home)
claude-code|.claude
pi|.pi
cursor|.config/[Cc]ursor

# project artifacts (relative to each discovered project)
claude-code|.claude
project|AGENTS.md
pi|.pi
pi-plans|.pi/approved-plan.md
aider|.aider.*
`

// syntheticMatcher compiles syntheticCatalog.
func syntheticMatcher(t *testing.T) *Matcher {
	t.Helper()
	tables, err := Parse([]byte(syntheticCatalog))
	if err != nil {
		t.Fatal(err)
	}
	matcher, err := NewMatcher(tables)
	if err != nil {
		t.Fatal(err)
	}
	return matcher
}

func TestSegmentExpressionMatchesAsTheAnalyzerGlobDoes(t *testing.T) {
	// Expected results from doubleagent.catalog.glob_segment_to_regex(...).fullmatch.
	cases := []struct {
		name, segment, subject string
		want                   bool
	}{
		{"a star matches a name with a suffix", "*.jsonl", "a.jsonl", true},
		{"a star never crosses a slash", "*", "a/b", false},
		{"a star takes a leading dot", "*", ".hidden", true},
		{"a question mark matches one character", "rollout-?.jsonl", "rollout-1.jsonl", true},
		{"a question mark matches one multibyte character", "?", "é", true},
		{"a question mark matches a newline", "?", "\n", true},
		{"a class matches its upper-case member", "[Zz]ed", "Zed", true},
		{"a class matches its lower-case member", "[Zz]ed", "zed", true},
		{"a class refuses a non-member", "[Zz]ed", "Xed", false},
		{"a negated class matches another character", "[!a]bc", "xbc", true},
		{"a negated class refuses its member", "[!a]bc", "abc", false},
		{"a range class matches a digit", "session-[0-9]*", "session-7x", true},
		{"spaces are literal", "Application Support", "Application Support", true},
		{"dots are literal", ".claude.json*", "xclaudejson", false},
		{"a star after literal dots matches a suffix", ".claude.json*", ".claude.json.backup", true},
		{"an unclosed bracket is literal", "a[b", "a[b", true},
		{"a backslash inside a class is literal", `[\x]`, `\`, true},
		{"matching is case-sensitive", ".claude", ".Claude", false},
		{"question then star needs one character", ".hermes?*", ".hermes", false},
		{"question then star takes a suffix", ".hermes?*", ".hermes-canary", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			expression := regexp.MustCompile(`\A(?:` + SegmentExpression(test.segment) + `)\z`)
			if got := expression.MatchString(test.subject); got != test.want {
				t.Fatalf("%q against %q = %v, want %v", test.segment, test.subject, got, test.want)
			}
		})
	}
}

func TestProjectAgentForTakesTheMostSpecificEntry(t *testing.T) {
	matcher := syntheticMatcher(t)
	cases := []struct{ name, rel, want string }{
		{"a file under an agent directory", ".claude/settings.local.json", "claude-code"},
		{"a file several agents read", "AGENTS.md", "project"},
		{"a file under a directory entry", ".pi/settings.json", "pi"},
		{"the nested entry wins", ".pi/approved-plan.md", "pi-plans"},
		{"a glob entry", ".aider.chat.history.md", "aider"},
		{"an unclaimed file", "README.md", ""},
		{"backslashes separate too", `.claude\settings.json`, "claude-code"},
		{"empty segments are dropped", "/.pi//approved-plan.md", "pi-plans"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := matcher.ProjectAgentFor(test.rel); got != test.want {
				t.Fatalf("ProjectAgentFor(%q) = %q, want %q", test.rel, got, test.want)
			}
		})
	}
}

func TestClaimsHomePathMatchesEachSegmentInPlace(t *testing.T) {
	matcher := syntheticMatcher(t)
	cases := []struct {
		name, rel string
		windows   bool
		want      string
	}{
		{"a path below an entry", ".claude/projects/x", false, "claude-code"},
		{"the entry itself", ".claude", false, "claude-code"},
		{"a class segment", ".config/Cursor/x", false, "cursor"},
		{"an unclaimed sibling", ".config/myproject", false, ""},
		{"case matters outside Windows", ".Claude/x", false, ""},
		{"Windows splits at backslashes and folds case", `\.CLAUDE\x`, true, "claude-code"},
		{"Windows does not split at slashes", ".claude/x", true, ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := matcher.ClaimsHomePath(test.rel, test.windows); got != test.want {
				t.Fatalf("ClaimsHomePath(%q, %v) = %q, want %q", test.rel, test.windows, got, test.want)
			}
		})
	}
}

func TestMatcherOfTheBundledCatalog(t *testing.T) {
	tables, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	matcher, err := NewMatcher(tables)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("every entry compiles", func(t *testing.T) {
		if len(matcher.Home) != len(tables.Catalog) || len(matcher.Project) != len(tables.ProjectCatalog) {
			t.Fatalf("compiled %d and %d entries of %d and %d", len(matcher.Home),
				len(matcher.Project), len(tables.Catalog), len(tables.ProjectCatalog))
		}
	})
	t.Run("a first segment of a home entry is recognised", func(t *testing.T) {
		if !matcher.MatchesFirstSegment(".claude") || matcher.MatchesFirstSegment("notes") {
			t.Fatal("first-segment test wrong for .claude or notes")
		}
	})
	t.Run("agents keep table order", func(t *testing.T) {
		agents := matcher.Agents()
		if len(agents) == 0 || agents[0] != "claude-code" {
			t.Fatalf("agents start %v", agents[:min(3, len(agents))])
		}
	})
}

func TestNewMatcherRefusesAGlobWithoutASegment(t *testing.T) {
	_, err := NewMatcher(&Tables{Catalog: []Entry{{Agent: "x", Glob: "/"}}})
	if err == nil {
		t.Fatal("no error for a glob of only a slash")
	}
}
