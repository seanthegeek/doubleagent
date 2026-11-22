package catalog

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// minimumTableLines is the fewest lines any table may hold, so an emptied or
// mis-sectioned table fails instead of passing as a table with nothing in it.
const minimumTableLines = 5

// tableHeaders lists the seven section headers in file order, with the section
// name each opens. A header typed slightly wrong is only a comment to the
// parser and would merge its lines into the table above, so each must appear.
var tableHeaders = []struct {
	section string
	header  string
}{
	{"home", "# home artifacts (agent|glob relative to home)"},
	{"project", "# project artifacts (relative to each discovered project)"},
	{"exclude", "# excluded unless --full"},
	{"secret", "# credential files (flagged secret, skipped with --no-secrets)"},
	{"docker", "# docker volumes (agent|volume name glob)"},
	{"relocation", "# relocation variables (agent|VARIABLE|home-relative path it replaces)"},
	{"startup", "# startup files searched for them (scope|glob; scope home, system or project)"},
}

// tableLine is one non-comment, non-blank line of a catalog text and the
// section the parser files it under.
type tableLine struct {
	section string
	number  int
	text    string
}

// tableLines returns the table lines of TEXT with the parser's own reading:
// white-space-only lines and comments are skipped, and only an exact header
// comment changes the section.
func tableLines(text string) []tableLine {
	var lines []tableLine
	section := "home"
	for index, line := range strings.Split(text, "\n") {
		switch {
		case isBlank(line):
		case strings.HasPrefix(line, "#"):
			if name, ok := sectionName(line); ok {
				section = name
			}
		default:
			lines = append(lines, tableLine{section, index + 1, line})
		}
	}
	return lines
}

// unclosedBracket reports whether TEXT holds a `[` with no closing `]`. A `]`
// first in a bracket list, or right after the `!`, is a member, as in the shells.
func unclosedBracket(text string) bool {
	for {
		start := strings.Index(text, "[")
		if start < 0 {
			return false
		}
		rest := strings.TrimPrefix(text[start+1:], "!")
		rest = strings.TrimPrefix(rest, "]")
		end := strings.Index(rest, "]")
		if end < 0 {
			return true
		}
		text = rest[end+1:]
	}
}

// globFormReason names the reason a table line holds a glob form the shells of
// the collector's matrix and the Go matcher read differently, or returns "".
func globFormReason(line string) string {
	switch {
	case strings.ContainsAny(line, `\^`):
		return "a backslash or a caret"
	case strings.Contains(line, "[:") || strings.Contains(line, "[.") || strings.Contains(line, "[="):
		return "a character class name, collating class or equivalence class"
	case unclosedBracket(line):
		return "an unclosed bracket"
	}
	return ""
}

// globFormDefects returns one finding for each table line of TEXT, in any
// table, whose glob holds a backslash, a caret, a character class name, a
// collating class, an equivalence class or an unclosed bracket.
func globFormDefects(text string) []string {
	var defects []string
	for _, line := range tableLines(text) {
		if reason := globFormReason(line.text); reason != "" {
			defects = append(defects, line.text+"  ("+reason+")")
		}
	}
	return defects
}

// secretGlobDefects returns one finding for each secret glob that holds a
// character the sh collector's compiled case pattern cannot hold (a bar,
// parenthesis, ampersand, semicolon, angle bracket, dollar sign, backtick,
// quote or backslash) or that starts with `#`.
func secretGlobDefects(globs []string) []string {
	var defects []string
	for _, glob := range globs {
		if strings.ContainsAny(glob, "|()&;<>$`\"'\\") || strings.HasPrefix(glob, "#") {
			defects = append(defects, glob)
		}
	}
	return defects
}

// tableSizes counts the parsed lines of each table by section name.
func tableSizes(tables *Tables) map[string]int {
	return map[string]int{
		"home": len(tables.Catalog), "project": len(tables.ProjectCatalog),
		"exclude": len(tables.Excludes), "secret": len(tables.SecretGlobs),
		"docker": len(tables.DockerVolumes), "relocation": len(tables.RelocationVars),
		"startup": len(tables.StartupFiles),
	}
}

// tableSizeDefects returns one finding for each table with fewer than
// minimumTableLines lines.
func tableSizeDefects(tables *Tables) []string {
	var defects []string
	for _, header := range tableHeaders {
		if size := tableSizes(tables)[header.section]; size < minimumTableLines {
			defects = append(defects, header.section+" table has "+strconv.Itoa(size)+" lines")
		}
	}
	return defects
}

// headerDefects returns one finding for each of the seven section headers that
// does not appear exactly once as a whole line of TEXT, or that appears out of
// file order.
func headerDefects(text string) []string {
	var defects []string
	lines := strings.Split(text, "\n")
	previous := -1
	for _, header := range tableHeaders {
		found := -1
		count := 0
		for index, line := range lines {
			if line == header.header {
				found, count = index, count+1
			}
		}
		if count != 1 {
			defects = append(defects, header.header+" appears "+strconv.Itoa(count)+" times")
		} else if found < previous {
			defects = append(defects, header.header+" is out of order")
		}
		previous = max(previous, found)
	}
	return defects
}

// hygieneDefects returns one finding for each line of TEXT that ends in white
// space or holds a carriage return or a tab, and for each table line repeated
// inside its own table.
func hygieneDefects(text string) []string {
	var defects []string
	for index, line := range strings.Split(text, "\n") {
		if strings.ContainsAny(line, "\r\t") || strings.TrimRight(line, " ") != line {
			defects = append(defects, "line "+strconv.Itoa(index+1)+" has trailing white space, a tab or a carriage return")
		}
	}
	seen := map[string]bool{}
	for _, line := range tableLines(text) {
		key := line.section + "\x00" + line.text
		if seen[key] {
			defects = append(defects, "line "+strconv.Itoa(line.number)+" repeats "+line.text)
		}
		seen[key] = true
	}
	return defects
}

// anchorDefects returns one finding for each catalog or project glob whose first
// path component is not anchored: a leading `*` is accepted only as the whole
// component `*` of `*/User/globalStorage/...`. Exclusion and secret globs are
// left out: the embedded tables open many of them with `*/`.
func anchorDefects(tables *Tables) []string {
	var defects []string
	for _, entry := range append(append([]Entry{}, tables.Catalog...), tables.ProjectCatalog...) {
		if strings.HasPrefix(entry.Glob, "*") && !strings.HasPrefix(entry.Glob, "*/User/globalStorage/") {
			defects = append(defects, entry.Agent+"|"+entry.Glob)
		}
	}
	return defects
}

// variableName is the form of an environment variable a RELOCATION_VARS line names.
var variableName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// relocationDefects returns one finding for each relocation line whose agent has
// white space around it, whose variable is not an upper-case environment
// variable name, or whose replaced path is absolute or ends in a slash; and for
// a variable repeated under the same agent.
func relocationDefects(tables *Tables) []string {
	var defects []string
	seen := map[RelocationVar]bool{}
	for _, line := range tables.RelocationVars {
		path := line.Replaces
		bad := !variableName.MatchString(line.Variable) ||
			strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") ||
			strings.TrimSpace(line.Agent) != line.Agent
		if bad {
			defects = append(defects, line.Agent+"|"+line.Variable+"|"+path)
		}
		key := RelocationVar{Agent: line.Agent, Variable: line.Variable}
		if seen[key] {
			defects = append(defects, "repeated "+line.Agent+"|"+line.Variable)
		}
		seen[key] = true
	}
	return defects
}

// startupDefects returns one finding for each startup line whose glob is an
// absolute path under scope home or project, or a relative one under scope
// system.
func startupDefects(tables *Tables) []string {
	var defects []string
	for _, line := range tables.StartupFiles {
		absolute := strings.HasPrefix(line.Glob, "/")
		if (line.Scope == "system") != absolute {
			defects = append(defects, line.Scope+"|"+line.Glob)
		}
	}
	return defects
}

// loadCrafted parses TEXT for a test, failing the test when it does not parse.
func loadCrafted(t *testing.T, text string) *Tables {
	t.Helper()
	tables, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

// loadEmbedded parses the embedded catalog for a test.
func loadEmbedded(t *testing.T) *Tables {
	t.Helper()
	tables, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

// expectFindings fails the test when FOUND is not exactly WANT's count, or when
// a wanted fragment is missing from the joined findings.
func expectFindings(t *testing.T, found []string, want ...string) {
	t.Helper()
	joined := strings.Join(found, "\n")
	if len(want) == 0 && len(found) != 0 {
		t.Fatalf("unexpected findings:\n%s", joined)
	}
	if len(want) != 0 && len(found) == 0 {
		t.Fatalf("no finding, wanted one naming %q", want)
	}
	for _, fragment := range want {
		if !strings.Contains(joined, fragment) {
			t.Errorf("findings %q lack %q", joined, fragment)
		}
	}
}

func TestEmbeddedCatalogHoldsOnlyGlobFormsEveryShellReadsAlike(t *testing.T) {
	expectFindings(t, globFormDefects(string(Text())))
}

func TestAGlobFormTheShellsReadDifferentlyIsFound(t *testing.T) {
	cases := []struct{ line, reason string }{
		{"agent|dir\\file", "a backslash or a caret"},
		{"agent|dir/[^a]*", "a backslash or a caret"},
		{"agent|dir/[[:alpha:]]*", "character class name"},
		{"agent|dir/[[.hyphen.]]x", "collating class"},
		{"agent|dir/[[=a=]]x", "equivalence class"},
		{"agent|dir/[abc", "an unclosed bracket"},
		{"agent|dir/[!]", "an unclosed bracket"},
		{"agent|dir/[]", "an unclosed bracket"},
		{"tmp/playwright/[a-", "an unclosed bracket"},
	}
	for _, testCase := range cases {
		t.Run(testCase.line, func(t *testing.T) {
			found := globFormDefects("# comment\nagent|.fine\n" + testCase.line + "\n")
			if len(found) != 1 || !strings.Contains(found[0], testCase.line) ||
				!strings.Contains(found[0], testCase.reason) {
				t.Fatalf("findings %q, want exactly one naming %q and %q",
					found, testCase.line, testCase.reason)
			}
		})
	}
}

func TestClosedBracketListsAndCommentsAreNotGlobDefects(t *testing.T) {
	text := "# a comment with [ and ^ and \\\n\nagent|[Cc]rew[Aa][Ii]\nagent|dir/[]a]x\n" +
		"agent|dir/[!]a]x\nagent|diagnostics_[0-9]*_*.json\nhermes-agent/[!.]*\n"
	expectFindings(t, globFormDefects(text))
}

func TestEmbeddedSecretGlobsHoldNoCharacterTheCasePatternCannot(t *testing.T) {
	expectFindings(t, secretGlobDefects(loadEmbedded(t).SecretGlobs))
}

func TestASecretGlobWithACasePatternSpecialOrALeadingHashIsFound(t *testing.T) {
	for _, glob := range []string{"a|b", "a(b", "a)b", "a&b", "a;b", "a<b", "a>b", "a$b",
		"a`b", `a"b`, "a'b", `a\b`, "#start"} {
		t.Run(glob, func(t *testing.T) {
			expectFindings(t, secretGlobDefects([]string{".fine/*", glob, "with space/x"}), glob)
		})
	}
	expectFindings(t, secretGlobDefects([]string{"dir with space/*.json", "a#b"}))
}

func TestEveryTableHoldsAtLeastFiveLines(t *testing.T) {
	expectFindings(t, tableSizeDefects(loadEmbedded(t)))
	empty := loadEmbedded(t)
	empty.SecretGlobs = empty.SecretGlobs[:4]
	empty.DockerVolumes = nil
	expectFindings(t, tableSizeDefects(empty), "secret table has 4 lines", "docker table has 0 lines")
}

func TestEverySectionHeaderAppearsOnceInFileOrder(t *testing.T) {
	text := string(Text())
	expectFindings(t, headerDefects(text))
	typo := strings.Replace(text, "# excluded unless --full", "# excluded unless --ful", 1)
	expectFindings(t, headerDefects(typo), "# excluded unless --full appears 0 times")
	doubled := text + "# docker volumes (agent|volume name glob)\n"
	expectFindings(t, headerDefects(doubled), "appears 2 times")
	swapped := tableHeaders[3].header + "\n" + tableHeaders[2].header + "\n"
	for _, header := range tableHeaders {
		if header.section != "exclude" && header.section != "secret" {
			swapped += header.header + "\n"
		}
	}
	expectFindings(t, headerDefects(swapped), "out of order")
}

func TestNoLineHasTrailingWhiteSpaceAndNoTableLineRepeats(t *testing.T) {
	expectFindings(t, hygieneDefects(string(Text())))
	expectFindings(t, hygieneDefects("a|b \n"), "line 1")
	expectFindings(t, hygieneDefects("a|b\r\n"), "line 1")
	expectFindings(t, hygieneDefects("# note \n"), "line 1")
	expectFindings(t, hygieneDefects("a|b\ta\n"), "line 1")
	expectFindings(t, hygieneDefects("a|b\nc|d\na|b\n"), "line 3 repeats a|b")
	repeatedAcrossTables := "a|b\n# excluded unless --full\na|b\n"
	expectFindings(t, hygieneDefects(repeatedAcrossTables))
}

func TestGlobsAreAnchoredOnAPathComponent(t *testing.T) {
	expectFindings(t, anchorDefects(loadEmbedded(t)))
	bad := loadCrafted(t, "a|*hermes*/auth.json\n"+
		"# project artifacts (relative to each discovered project)\np|*/x\n")
	expectFindings(t, anchorDefects(bad), "a|*hermes*/auth.json", "p|*/x")
	ok := loadCrafted(t, "a|*/User/globalStorage/x/*\nb|.hermes*/auth.json\n")
	expectFindings(t, anchorDefects(ok))
}

func TestRelocationLinesNameAnUpperCaseVariableAndAHomeRelativePath(t *testing.T) {
	expectFindings(t, relocationDefects(loadEmbedded(t)))
	header := "# relocation variables (agent|VARIABLE|home-relative path it replaces)\n"
	cases := []struct{ name, line, want string }{
		{"a lower-case variable", "a|a_home|.a", "a|a_home|.a"},
		{"an absolute replaced path", "a|A_HOME|/etc/a", "a|A_HOME|/etc/a"},
		{"a replaced path ending in a slash", "a|A_HOME|.a/", "a|A_HOME|.a/"},
		{"an agent with white space around it", " a|A_HOME|.a", " a|A_HOME|.a"},
		{"a variable repeated under one agent", "a|A_HOME|.a\na|A_HOME|.b", "repeated a|A_HOME"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			expectFindings(t, relocationDefects(loadCrafted(t, header+testCase.line+"\n")), testCase.want)
		})
	}
	shared := "a|A_HOME|.a\nb|A_HOME|.b\nshared|XDG_CONFIG_HOME|.config\nc|C_EXTRA|\nc|C_ROOT|.\n"
	expectFindings(t, relocationDefects(loadCrafted(t, header+shared)))
}

func TestStartupFilesAreAbsoluteExactlyUnderScopeSystem(t *testing.T) {
	expectFindings(t, startupDefects(loadEmbedded(t)))
	header := "# startup files searched for them (scope|glob; scope home, system or project)\n"
	expectFindings(t, startupDefects(loadCrafted(t, header+"home|/etc/x\nsystem|etc/y\nproject|.z\n")),
		"home|/etc/x", "system|etc/y")
}
