package analyze

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/gitmeta"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/rowfilter"
)

// controlValues are evidence texts holding what a terminal acts on, each
// with the text shown must give.
func controlValues() []struct{ name, text, want string } {
	return []struct{ name, text, want string }{
		{"an escape sequence", "evil\x1b]0;pwned\x07\x1b[2Jhost", `evil\x1b]0;pwned\x07\x1b[2Jhost`},
		{"a carriage return", "a\rb", `a\x0db`},
		{"a lone line feed inside a value", "a\nb", `a\x0ab`},
		{"a tab inside a value", "a\tb", `a\x09b`},
		{"DEL", "a\x7fb", `a\x7fb`},
		{"the one-character CSI of C1", "a\u009b2Jb", `a\u009b2Jb`},
		{"a byte that is not UTF-8", "a\x9bb", `a\x9bb`},
		{"a combining and a wide character", "e\u0301\u4e16", "e\u0301\u4e16"},
		{"plain text", "alice", "alice"},
	}
}

func TestShownShowsEachControlCharacterVisibly(t *testing.T) {
	for _, test := range controlValues() {
		t.Run(test.name, func(t *testing.T) {
			if got := shown(test.text); got != test.want {
				t.Errorf("shown(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

// wantShownOnly fails the test unless output holds want and no byte a
// terminal acts on but the line ends and separators the writer adds.
func wantShownOnly(t *testing.T, output, want string, separators string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Errorf("output does not hold %q:\n%q", want, output)
	}
	for i, char := range output {
		if char < 0x20 && !strings.ContainsRune("\n"+separators, char) || char == 0x7f ||
			char >= 0x80 && char <= 0x9f || char == utf8.RuneError {
			t.Errorf("output holds %q at %d:\n%q", char, i, output)
			return
		}
	}
}

func TestEveryTextWriterShowsControlCharactersVisibly(t *testing.T) {
	for _, value := range controlValues() {
		t.Run(value.name, func(t *testing.T) {
			home := &input.Home{User: value.text, Original: "/home/" + value.text}
			collection := &input.Collection{Source: value.text, Kind: input.KindCollected, Host: value.text,
				Notes: []string{value.text}, Problems: []string{value.text}, Homes: []*input.Home{home},
				Artifacts: []*input.Artifact{{Agent: value.text, Original: value.text, Home: home,
					Manifest: jsonvalue.NewNull()}}}
			detected := detection{projects: []projectEntry{{path: value.text, users: []string{value.text},
				agents: []agentFiles{{fileCounts: fileCounts{agent: value.text, files: 1}}},
				git:    projectGit{remotes: []gitmeta.Remote{{Name: value.text, URLs: []string{value.text}}}}}}}
			var detect bytes.Buffer
			printDetect(&detect, detected, collection, map[string]bool{}, true)
			wantShownOnly(t, detect.String(), "host:    "+value.want+"\n", "\t")
			wantShownOnly(t, detect.String(), value.want+"\t"+value.want+"\t"+value.want+"\n", "\t")
			wantShownOnly(t, detect.String(), "  remotes: "+value.want+" "+value.want+"\n", "\t")
			run := &timelineRun{collection: collection, problems: []string{value.text},
				filter: &rowfilter.RowFilter{}, options: TimelineOptions{Match: []string{value.text}}}
			var header bytes.Buffer
			run.printHeader(&header)
			wantShownOnly(t, header.String(), "problem:   "+value.want+"\n", "")
			wantShownOnly(t, header.String(), "note:      "+value.want+"\n", "")
			var fleet bytes.Buffer
			printFleet(&fleet, [][]string{{value.text, "", "", "", value.text, value.text, "1", "", "", value.text,
				"", ""}}, value.text)
			wantShownOnly(t, fleet.String(), value.want, "")
			var errors bytes.Buffer
			printError(context.Background(), &errors, value.text)
			wantShownOnly(t, errors.String(), "error: "+value.want+"\n", "")
		})
	}
}

func TestATableIsAlignedByRunesOfTheShownText(t *testing.T) {
	var out bytes.Buffer
	printHomeTable(&out, []tableRow{{user: "a\x1b", home: "/h", agent: "x"}, {user: "e\u0301", home: "/h",
		agent: "x"}})
	lines := strings.Split(out.String(), "\n")
	first, second := strings.Index(lines[2], "/h"), strings.Index(lines[3], "/h")
	if utf8.RuneCountInString(lines[2][:first]) != utf8.RuneCountInString(lines[3][:second]) {
		t.Errorf("columns differ:\n%s\n%s", lines[2], lines[3])
	}
}

func TestTheCommandsPrintNoControlCharacterFromTheEvidence(t *testing.T) {
	const hostile = "evil\x1b]0;pwned\x07\x1b[2J\rhost\u009b"
	work := t.TempDir()
	// The home's directory is made on disk, and Windows refuses a control
	// character in a name; there the manifest's user and agent carry them.
	home := "/home/al\x1b[2Jice"
	if runtime.GOOS == "windows" {
		home = "/home/alice"
	}
	collection := writeCollection(t, filepath.Join(work, "col"), []jsonvalue.Member{
		member("hostname", jsonvalue.NewString(hostile))},
		[]collectedFile{{"al\x1b[2Jice", home, "claude\x07code", ".claude/x", "x"}})
	inventory := filepath.Join(work, "inv.jsonl")
	writeFile(t, inventory, `{"type":"host","host":"`+"evil\\u001b[2J"+`"}`+"\n"+
		`{"type":"agent","agent":"claude-code\u001b[2J","last":"x\u0007"}`+"\n")
	for _, args := range [][]string{
		{"detect", collection},
		{"detect", "--files", collection},
		{"timeline", collection, filepath.Join(work, "out")},
		{"fleet", "-o", filepath.Join(work, "f.csv"), inventory},
	} {
		t.Run(args[0], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			Run(context.Background(), args, testEnvironment(fakeParsers("claude-code")...), &stdout, &stderr)
			separators := ""
			if args[1] == "--files" {
				separators = "\t"
			}
			wantShownOnly(t, stdout.String()+stderr.String(), "", separators)
		})
	}
	data, err := os.ReadFile(filepath.Join(work, "out", InventoryFile))
	if err != nil || !strings.Contains(string(data), `"host": "evil\u001b]0;pwned\u0007\u001b[2J\rhost`) {
		t.Errorf("inventory.json does not carry the host as written: %v\n%s", err, data)
	}
}

// failingWriter fails every write.
type failingWriter struct{}

// Write fails.
func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestAnOutputThatCannotBeWrittenIsAnError(t *testing.T) {
	work := t.TempDir()
	for _, args := range [][]string{{"detect", "--json", work}, {"catalog"}, {"catalog", "--agents"},
		{"detect", work}} {
		var stderr bytes.Buffer
		code := Run(context.Background(), args, testEnvironment(), failingWriter{}, &stderr)
		if code != ExitError || !strings.HasPrefix(stderr.String(), "error: the output could not be written: ") {
			t.Errorf("%v: exit %d, stderr %q", args, code, stderr.String())
		}
	}
}

func TestAnAgentNoParserReadsIsAMistake(t *testing.T) {
	work := t.TempDir()
	output := filepath.Join(work, "out")
	code, stdout, stderr := runCommand(testEnvironment(fakeParsers("pi")...), "timeline", "--agent", "pi",
		"--agent", "claude-cod", work, output)
	if code != ExitError || stdout != "" || stderr != "error: --agent claude-cod: no parser reads this agent\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if lstatExists(output) {
		t.Error("the output directory was made")
	}
}
