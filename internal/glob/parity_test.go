//go:build parity

package glob

import (
	"bytes"
	"math/rand/v2"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// expandScript is expand_glob's body with a byte-1 separator, since the names
// under test hold newlines.
const expandScript = `cd -P "${1:-/}" 2>/dev/null || exit 0
IFS='
'
for match in $2; do
  match=$1/$match
  if [ -e "$match" ] || [ -L "$match" ]; then printf '%s\001' "$match"; fi
done`

// caseScript reads pattern, byte 1, subject lines and prints 1 or 0 for each,
// as test_excluded and is_secret test a path.
const caseScript = `IFS=$(printf '\001')
while read -r pattern subject; do
  case "$subject" in $pattern) echo 1 ;; *) echo 0 ;; esac
done < "$1"`

// runDash runs script under dash with LC_ALL=C and returns its stdout.
func runDash(t *testing.T, script string, args ...string) []byte {
	t.Helper()
	dash, err := exec.LookPath("dash")
	if err != nil {
		t.Fatalf("the oracle needs dash: %v", err)
	}
	command := exec.Command(dash, append([]string{"-c", script, "sh"}, args...)...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("dash: %v", err)
	}
	return output
}

func TestExpandAgreesWithDash(t *testing.T) {
	base := buildTree(t, "ro[o]t*")
	patterns := []string{
		"*", ".*", "*/*", "*/file", "link-dir/*", "*/.dot", ".h*/auth.json", "[ab]",
		"[!a]*", "?", "??", "*.txt", "dir/sub/file", "missing", "missing*", "x*", "*y",
		"n*", "*\n", "sp*/f", "sp ace/*", "*/", "d*/", "dir/", "dangling", "dang*",
		"link-*/sub", "link-*/sub/*", "../*", "../out*/*", `q\*`, "q*", `\[ab]`, "[[]ab]",
		"a.txt\nb.txt", "\n*.txt\n\n", "", "[a-c]*", "[[:alpha:]]*", "[!.]*", ".[!.]*",
		"*/[!.]*", "dir/*/*", "*/sub", "file-not-dir/*", "[.]dot", "?dot", `\.h*`,
		"dir/.*", "dir/..", "./*", "dir/./sub", "*/../a.txt", "[]", "x[", "x[*", "[!]",
		`dir\/sub`, `d*\/sub`, "sp ace", "é*", "*[é]*", "nl*", "unreadable/*",
		"unreadable/x", "unre*/x", "*/*/*",
	}
	for _, pattern := range patterns {
		output := runDash(t, expandScript, base, pattern)
		want := []string{}
		if len(output) > 0 {
			want = strings.Split(strings.TrimSuffix(string(output), "\x01"), "\x01")
		}
		got := Expand(base, pattern)
		if got == nil {
			got = []string{}
		}
		if !slices.Equal(got, want) {
			t.Errorf("Expand(%q):\n got %q\nwant %q", pattern, got, want)
		}
	}
}

func TestMatchPathAgreesWithDash(t *testing.T) {
	var patterns, subjects, lines []string
	add := func(pattern, subject string) {
		patterns, subjects = append(patterns, pattern), append(subjects, subject)
		lines = append(lines, pattern+"\x01"+subject)
	}
	for _, pattern := range curatedPatterns() {
		for _, subject := range curatedSubjects() {
			add(pattern, subject)
		}
	}
	addGeneratedCases(add)
	file := t.TempDir() + "/cases"
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	answers := bytes.Fields(runDash(t, caseScript, file))
	if len(answers) != len(lines) {
		t.Fatalf("dash gave %d answers for %d cases", len(answers), len(lines))
	}
	failures := 0
	for index, answer := range answers {
		want := string(answer) == "1"
		if got := MatchPath(patterns[index], subjects[index]); got != want && failures < 50 {
			failures++
			t.Errorf("MatchPath(%q, %q) = %v, dash says %v",
				patterns[index], subjects[index], got, want)
		}
	}
}

// addGeneratedCases adds 25 random subjects for each of 4000 random patterns
// built from the portable pieces: wildcards, closed bracket lists, ranges,
// negations and the one character class the tables may use. Shells differ on
// an unclosed bracket, a backslash, a caret, a collating or equivalence class
// and a class next to a range, and no table holds those, so they are not
// generated; the same each run.
func addGeneratedCases(add func(pattern, subject string)) {
	tokens := strings.Fields(`a b - ] ! * ? : . / z [ab] [!a] [a-z] []a] [!.] [[:alpha:]] ` +
		`[0-9a-f] [Zz] [!]a] [a-]`)
	letters := strings.Fields(`a b z - ] [ ! ^ : . / \ =`)
	random := rand.New(rand.NewPCG(1, 2))
	for range 4000 {
		pattern := randomText(random, tokens, 1+random.IntN(7))
		for range 25 {
			add(pattern, randomText(random, letters, random.IntN(4)))
		}
	}
}

// curatedPatterns are the constructs the tables use, in the forms POSIX
// defines and every shell of the collector's matrix agrees on.
func curatedPatterns() []string {
	return []string{
		"*", "?", "a*", "*a", "a?c", "a*b*c", "[ab]", "[!ab]", "[a-c]", "[!a-c]", "[]a]",
		"[!]a]", "[a-]", "[]-a]", "[z-a]", "[[:alpha:]]", "[![:digit:]]", "[]", "[!]",
		"*/x", "*.json", ".h*/auth.json", "[!.]*", ".[!e]*", "[0-9a-f][0-9a-f]", "a/*",
		"*/[Zz]ed/node", "[.]*", "",
	}
}

// curatedSubjects are the paths the curated patterns are tried against.
func curatedSubjects() []string {
	return []string{
		"", "a", "b", "c", "z", "-", "]", "[", "^", `\`, `\^`, "*", "?", ".", "x", `x\`,
		"ab", "abc", "aXbYc", "a/x", "a/b/x", ".hidden", ".h/auth.json", ".env", ".e",
		"0f", "x/Zed/node", "y/zed/node", "a/zed/node/z", "a.json", "a/b.json", "[a]",
		"x[", "[a", "!", "1",
	}
}

// randomText joins count random tokens.
func randomText(random *rand.Rand, tokens []string, count int) string {
	var text strings.Builder
	for range count {
		text.WriteString(tokens[random.IntN(len(tokens))])
	}
	return text.String()
}

func TestMatchPathAcrossNewlinesAgreesWithDash(t *testing.T) {
	script := `case "$2" in $1) echo 1 ;; *) echo 0 ;; esac`
	patterns := []string{"*", "a*", "*b", "a?b", "a[!x]b", "a[[:space:]]b", "*\n", "a\n*"}
	subjects := []string{"a\nb", "a\n", "\n", "a/\nb", "ab\n"}
	for _, pattern := range patterns {
		for _, subject := range subjects {
			want := string(bytes.TrimSpace(runDash(t, script, pattern, subject))) == "1"
			if got := MatchPath(pattern, subject); got != want {
				t.Errorf("MatchPath(%q, %q) = %v, dash says %v", pattern, subject, got, want)
			}
		}
	}
}
