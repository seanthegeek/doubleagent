package parserkit

import (
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

func TestARelPatternIgnoresTheCaseOfASCIILettersOnly(t *testing.T) {
	quoted := `\A` + regexp.QuoteMeta("Application Support/Claude (Beta)") + `/(.+)\z`
	tests := []struct {
		name       string
		expression string
		rel        string
		want       []string
	}{
		{"a literal written in mixed case matches any ASCII case", `\A\.Claude/Projects/x\.jsonl\z`,
			".claude/PROJECTS/X.JSONL", []string{".claude/PROJECTS/X.JSONL"}},
		{"a capture keeps the spelling of the rel given", `\A\.claude/([^/]+)/(?P<session>[^/]+)\.jsonl\z`,
			".Claude/My-Proj/AbC.JSONL", []string{".Claude/My-Proj/AbC.JSONL", "My-Proj", "AbC"}},
		{"KELVIN SIGN does not match k", `\Ak\z`, "K", nil},
		{"KELVIN SIGN does not match K", `\AK\z`, "K", nil},
		{"LONG S does not match s", `\As\z`, "ſ", nil},
		{"a class of small letters does not match KELVIN SIGN", `\A[a-z]\z`, "K", nil},
		{"a class of capitals does not match LONG S", `\A[A-Z]\z`, "ſ", nil},
		{"a non-ASCII letter does not match its other case", `\Aé\z`, "É", nil},
		{"a non-ASCII letter matches itself", `\Aé\z`, "é", []string{"é"}},
		{"a class of capitals matches small letters", `\A[A-Z]+\z`, "abC", []string{"abC"}},
		{"a class of small letters and digits matches capitals", `\A[a-f0-9]+\z`, "DEADbeef09",
			[]string{"DEADbeef09"}},
		{"a class of small letters matches no letter outside it", `\A[a-f]\z`, "G", nil},
		{"a negated class without letters matches letters in any case", `\A[^/]+\z`, "AbC", []string{"AbC"}},
		{"a negated class without letters still leaves out its own", `\A[^/]+\z`, "Ab/C", nil},
		{"a class negating the capitals matches a capital", `\A[^A-Z]\z`, "Q", []string{"Q"}},
		{"a class negating the small letters matches a small letter", `\A[^a-z]\z`, "q", []string{"q"}},
		{"a class negating both cases matches no letter", `\A[^A-Za-z]\z`, "q", nil},
		{"a class negating both cases matches a digit", `\A[^A-Za-z]\z`, "1", []string{"1"}},
		{"a caret and a dollar anchor at the ends", `^A/B$`, "a/b", []string{"a/b"}},
		{"a caret does not match inside", `^B`, "a/b", nil},
		{"a dollar does not match before a final newline", `A$`, "a\n", nil},
		{"an anchored end does not match inside", `\Ab\z`, "bB", nil},
		{"an alternation matches each branch in any case", `\A(?:Cursor|Code)/User\z`, "code/USER",
			[]string{"code/USER"}},
		{"an alternation matches no other name", `\A(?:Cursor|Code)/User\z`, "Codex/User", nil},
		{"quoted text with capitals matches in any case", quoted, "application support/CLAUDE (beta)/Log.txt",
			[]string{"application support/CLAUDE (beta)/Log.txt", "Log.txt"}},
		{"digits and non-space escapes keep their meaning", `\A\d+\S\z`, "12X", []string{"12X"}},
		{"a space is not a non-space", `\A\d+\S\z`, "12 ", nil},
		{"a group that took no part is empty", `\A(a)?(B)\z`, "b", []string{"b", "", "b"}},
		{"a letter pair the parser folds matches both cases", `\A[Éé][Kk]\z`, "ÉK", []string{"ÉK"}},
		{"a letter pair the parser folds does not match KELVIN SIGN", `\A[Éé][Kk]\z`, "éK", nil},
		{"an ASCII pair beside a non-ASCII pair matches both cases", `\A[Aa][Éé]\z`, "aÉ", []string{"aÉ"}},
		{"a byte that is not UTF-8 keeps the places of the groups", `\A([^/]+)/X\z`, "\xffA/x",
			[]string{"\xffA/x", "\xffA"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pattern := MustRelPattern(test.expression)
			if got := pattern.FindStringSubmatch(test.rel); !slices.Equal(got, test.want) || (got == nil) != (test.want == nil) {
				t.Errorf("FindStringSubmatch(%q) = %q, want %q", test.rel, got, test.want)
			}
			if got := pattern.MatchString(test.rel); got != (test.want != nil) {
				t.Errorf("MatchString(%q) = %v, want %v", test.rel, got, test.want != nil)
			}
		})
	}
}

func TestARelPatternNamesItsGroupsAndPlacesItsMatch(t *testing.T) {
	pattern := MustRelPattern(`Sessions/(?P<session>[^/]+)/(x)`)
	if got := pattern.SubexpIndex("session"); got != 1 {
		t.Errorf("SubexpIndex(session) = %d, want 1", got)
	}
	if got := pattern.SubexpIndex("missing"); got != -1 {
		t.Errorf("SubexpIndex(missing) = %d, want -1", got)
	}
	if got := pattern.SubexpNames(); !slices.Equal(got, []string{"", "session", ""}) {
		t.Errorf("SubexpNames() = %q", got)
	}
	if got := pattern.FindStringIndex("a/SESSIONS/One/X"); !slices.Equal(got, []int{2, 16}) {
		t.Errorf("FindStringIndex = %v, want [2 16]", got)
	}
	if got := pattern.FindStringIndex("a/sessions/one"); got != nil {
		t.Errorf("FindStringIndex = %v, want nil", got)
	}
}

func TestMustRelPatternRefusesTheCaseFlagAndABadExpression(t *testing.T) {
	for _, expression := range []string{`(?i)k`, `(?i:a)b`, `(?si)x`, `a[`} {
		t.Run(expression, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered == nil || !strings.Contains(recovered.(string), "MustRelPattern") {
					t.Errorf("MustRelPattern(%q) gave %v, want a panic naming it", expression, recovered)
				}
			}()
			MustRelPattern(expression)
		})
	}
}

func TestARelPatternMatchAllocatesOnlyToLowerARelWithCapitals(t *testing.T) {
	pattern := MustRelPattern(`\ALibrary/Application Support/Claude/[^/]+\.json\z`)
	tests := []struct {
		name string
		rel  string
		want float64
	}{
		{"a rel in small letters allocates nothing", "library/application support/claude/config.json", 0},
		{"a rel with capitals allocates its lowered copy", "Library/Application Support/Claude/config.json", 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allocations := testing.AllocsPerRun(100, func() {
				if !pattern.MatchString(test.rel) {
					t.Fatalf("%q does not match", test.rel)
				}
			})
			if allocations > test.want {
				t.Errorf("got %v allocations, want at most %v", allocations, test.want)
			}
		})
	}
}

func TestARelPatternMatchesAsTheLoweredTreePrintedByGoWould(t *testing.T) {
	expressions := []string{
		`\A\.claude/projects/(?P<project>[^/]+)/(?P<session>[^/]+)\.jsonl\z`,
		`(?:^|/)logs/(?:[^/]+/)+rjmacarthy\.twinny/Twinny\.log(?:_[0-9]+)?\z`,
		`(^|/)shell_gpt/cache/[0-9a-f]{32}\z`, `\A[0-9]{2,4}/[a-f]{3,}/x\z`,
		`\A(.*)[\\/](?:cron|triggers)[\\/]runs[\\/][^\\/]+\z`, `\A(.*?)/(.+?)(x??)(y*?)(z+?)\z`,
		`\A(?:(?P<state>a|B(?:c|D))|e)+\z`, `\A[[:alpha:]][[:^digit:]]\pL\PL\w\W\s\S\d\D\z`,
		`(?s)\A.+\z`, `\A.+\z`, `(?m)^A$`, `\bWord\B`, `A$`, `\Aé[Éé][Kk]Ä\z`, `\A()|(?:)\z`,
		`\A[^\x00-\x{10FFFF}]\z`, `\Ax{0}\z`, `\A(?:ab){1}c{2,}d{0,3}?\z`, `\A[\-\]\^\\]+\z`,
	}
	inputs := []string{"", "a", "x", "\n", "A\nB", ".claude/projects/My-Proj/AbC.jsonl", "logs/1/2/rjmacarthy.twinny/Twinny.log_42",
		"shell_gpt/cache/" + strings.Repeat("Ab", 16), "123/abcd/x", "home\\cron/runs/x", "a/b/c/xyyzz",
		"aBcD", "BdE", "Ax9Éx_!\t5z", "word ab", "éÉkä", "ä", "ababcccd", "ab ccc", "-]^\\", "x\xffy"}
	for _, expression := range expressions {
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			tree, err := syntax.Parse(expression, syntax.Perl)
			if err != nil {
				t.Fatal(err)
			}
			lowerTree(tree)
			reference := regexp.MustCompile(tree.String())
			pattern := MustRelPattern(expression)
			for _, input := range inputs {
				lowered := textlines.LowerASCII(input)
				got, want := pattern.lowered.FindStringSubmatchIndex(lowered), reference.FindStringSubmatchIndex(lowered)
				if !slices.Equal(got, want) {
					t.Errorf("on %q got %v, want %v", input, got, want)
				}
			}
			if got, want := pattern.SubexpNames(), reference.SubexpNames(); !slices.Equal(got, want) {
				t.Errorf("SubexpNames() = %q, want %q", got, want)
			}
		})
	}
}

func TestMustRelPatternRefusesAnOperatorItCannotWrite(t *testing.T) {
	defer func() {
		if recovered, _ := recover().(string); !strings.Contains(recovered, "MustRelPattern") {
			t.Errorf("writeTree gave %q, want a panic naming MustRelPattern", recovered)
		}
	}()
	var text strings.Builder
	writeTree(&text, &syntax.Regexp{Op: 200}, "x")
}

// compileTime returns the least time of five runs of compiling expression
// a hundred times, the least so that a pause of the machine is left out.
func compileTime(expression string) time.Duration {
	least := time.Duration(1<<63 - 1)
	for range 5 {
		start := time.Now()
		for range 100 {
			MustRelPattern(expression)
		}
		least = min(least, time.Since(start))
	}
	return least
}

func TestANegatedClassCompilesAboutAsFastAsAClassOfOneRune(t *testing.T) {
	negated, single := compileTime(`\A[^/]+\z`), compileTime(`\A[/]+\z`)
	if negated > 20*single {
		t.Errorf("[^/] took %v a hundred times, [/] %v: more than 20 times as long", negated, single)
	}
}

// A flag group that names the i flag is found wherever it stands and
// however its flags are ordered; other flag groups and plain groups are not.
func TestOnlyAFlagGroupNamingTheCaseFlagIsRefused(t *testing.T) {
	tests := []struct {
		expression string
		want       bool
	}{
		{`(?i)k`, true}, {`a(?si:x)`, true}, {`(?s)a(?-i)b`, true}, {`(?U)(?mi)`, true},
		{`(?s)\A.+\z`, false}, {`(?P<id>[a-z]+)`, false}, {`(?:pi|ini)`, false}, {`(?i`, false}, {`(?`, false},
	}
	for _, test := range tests {
		if got := namesCaseFlag(test.expression); got != test.want {
			t.Errorf("namesCaseFlag(%q) = %v, want %v", test.expression, got, test.want)
		}
	}
}
