package catalog

import (
	"slices"
	"testing"
)

// syntheticCatalog is the small catalog of the project and claim tests,
// with one home entry whose segment is a class.
const syntheticCatalog = `# home artifacts (agent|glob relative to home)
claude-code|.claude
pi|.pi
cursor|.config/[Cc]ursor
kiro|.kiro

# project artifacts (relative to each discovered project)
claude-code|.claude
project|AGENTS.md
pi|.pi
pi-plans|.pi/approved-plan.md
aider|.aider.*
`

// syntheticMatcher builds the matcher of syntheticCatalog.
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

func TestASegmentMatchesByTheRuleCollectExpandsWith(t *testing.T) {
	// A segment matches one whole name by glob.MatchSegment: ASCII case
	// ignored and no other fold, a star or question mark never taking a
	// leading dot the segment does not write.
	cases := []struct {
		name, segment, subject string
		want                   bool
	}{
		{"a star matches a name with a suffix", "*.jsonl", "a.jsonl", true},
		{"ASCII case is ignored", "goose", "Goose", true},
		{"ASCII case is ignored in a class", "[c]ursor", "Cursor", true},
		{"a Kelvin sign is not a k", ".kiro", ".\u212Airo", false},
		{"a long s is not an s", ".skills", ".\u017Fkills", false},
		{"a star leaves a leading dot alone", "*", ".hidden", false},
		{"a question mark leaves a leading dot alone", "?claude", ".claude", false},
		{"a written leading dot matches one", ".claude-*", ".claude-work", true},
		{"a question mark matches one byte", "?", "\u00e9", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			pattern := Pattern{Segments: []string{test.segment}}
			if got := pattern.MatchSegment(0, test.subject); got != test.want {
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
	cases := []struct{ name, rel, separator, want string }{
		{"a path below an entry", ".claude/projects/x", "/", "claude-code"},
		{"the entry itself", ".claude", "/", "claude-code"},
		{"a class segment", ".config/Cursor/x", "/", "cursor"},
		{"an unclaimed sibling", ".config/myproject", "/", ""},
		{"ASCII case is ignored", ".Claude/x", "/", "claude-code"},
		{"a Kelvin sign is not ASCII case", "\\.\u212Airo\\x", `\`, ""},
		{"a Windows path folds ASCII case", `\.KIRO\x`, `\`, "kiro"},
		{"a backslash separator splits there", `\.CLAUDE\x`, `\`, "claude-code"},
		{"a backslash separator does not split at slashes", ".claude/x", `\`, ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := matcher.ClaimsHomePath(test.rel, test.separator); got != test.want {
				t.Fatalf("ClaimsHomePath(%q, %q) = %q, want %q", test.rel, test.separator, got, test.want)
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
	t.Run("every entry is split", func(t *testing.T) {
		if len(matcher.Home) != len(tables.Catalog) || len(matcher.Project) != len(tables.ProjectCatalog) {
			t.Fatalf("split %d and %d entries of %d and %d", len(matcher.Home),
				len(matcher.Project), len(tables.Catalog), len(tables.ProjectCatalog))
		}
	})
	t.Run("a first segment of a home entry is recognised", func(t *testing.T) {
		if !matcher.MatchesFirstSegment(".claude") || matcher.MatchesFirstSegment("notes") {
			t.Fatal("first-segment test wrong for .claude or notes")
		}
	})
	t.Run("agents come in table order, which the loader sorts by name", func(t *testing.T) {
		agents := matcher.Agents()
		if len(agents) == 0 || agents[0] != "agent-zero" || !slices.IsSorted(agents) {
			t.Fatalf("agents start %v, sorted: %v", agents[:min(3, len(agents))], slices.IsSorted(agents))
		}
	})
}

func TestNewMatcherRefusesAGlobWithoutASegment(t *testing.T) {
	_, err := NewMatcher(&Tables{Catalog: []Entry{{Agent: "x", Glob: "/"}}})
	if err == nil {
		t.Fatal("no error for a glob of only a slash")
	}
}
