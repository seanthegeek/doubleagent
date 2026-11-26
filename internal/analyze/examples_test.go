package analyze

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The output page shows the commands' output over the example collection
// in blocks between the comments "<!-- example output: NAME -->" and
// "<!-- end example output: NAME -->", each with the command line that
// writes it in a one-line sh block just above. This test runs each command
// line of exampleCases and fails when its block differs or the page shows
// another command; DOUBLEAGENT_UPDATE_GOLDEN=1 rewrites the text inside the
// blocks' fences and nothing else.

// examplePage is the page whose example blocks the test keeps true.
const examplePage = "../../docs/timeline-output.md"

// exampleArchive is where the page places the example collection: the
// archive a collection run on the host wsdev14 names, in a case directory.
const exampleArchive = "/cases/wsdev14/wsdev14_20261015T165610Z_agent-artifacts.tar.gz"

// exampleVersion is the version the example output names: a development
// build's, so the page claims no release.
const exampleVersion = "0.0.0-dev"

// exampleCase is one example block and the command line that writes it.
type exampleCase struct {
	name string
	// command is the command line as the page shows it.
	command string
	// file is the output file under out/ the block shows, or "" for the
	// command's standard output.
	file string
}

// exampleMatch is the filter of the timeline.jsonl example: a prompt the
// history file alone keeps, the calls around calc.py with their results
// and decisions, and the model switches.
const exampleMatch = `'calc\.py|^model: .* -> |older prompt'`

// exampleCases returns every example block of the page, in page order.
func exampleCases() []exampleCase {
	timeline := "doubleagent timeline " + exampleArchive + " out"
	return []exampleCase{
		{name: "detect-text", command: "doubleagent detect " + exampleArchive},
		{name: "detect-json", command: "doubleagent detect --json " + exampleArchive},
		{name: "timeline-stdout", command: timeline},
		{name: "timeline-jsonl", file: timelineFile, command: "doubleagent timeline --agent claude-code " +
			"--agent codex-cli --match " + exampleMatch + " " + exampleArchive + " out"},
		{name: "sessions-jsonl", file: sessionsFile, command: timeline},
		{name: "inventory-json", file: inventoryFile, command: timeline},
	}
}

func TestTheOutputPageShowsWhatEachExampleCommandPrints(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the page shows the POSIX paths of a Linux or macOS workstation")
	}
	page := readExamplePage(t)
	archive, err := osfile.ReadFile(filepath.Join(goldenFixtures, "example.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	writeFile(t, filepath.Join(work, filepath.FromSlash(exampleArchive)), string(archive))
	outputs := map[string]string{}
	for _, test := range exampleCases() {
		t.Run(test.name+" is the output of the command shown above it", func(t *testing.T) {
			block, err := findExampleBlock(page, test.name)
			if err != nil {
				t.Fatal(err)
			}
			if block.command != test.command {
				t.Fatalf("the page shows\n  %s\nabove the block; the test runs\n  %s", block.command, test.command)
			}
			outputs[test.name] = runExample(t, work, test)
			if os.Getenv(updateGoldenVariable) != "1" && block.body != outputs[test.name] {
				t.Errorf("block %s differs from the command's output (- page, + output):\n%s",
					test.name, lineDiff(block.body, outputs[test.name]))
			}
		})
	}
	if os.Getenv(updateGoldenVariable) == "1" && !t.Failed() {
		writeExamplePage(t, page, outputs)
	}
}

func TestEveryExampleBlockOfThePageHasACase(t *testing.T) {
	page := readExamplePage(t)
	known := map[string]bool{}
	for _, test := range exampleCases() {
		known[test.name] = true
	}
	for _, line := range strings.Split(page, "\n") {
		name, found := strings.CutPrefix(line, exampleStart)
		if found && !known[strings.TrimSuffix(name, " -->")] {
			t.Errorf("the page has a block %q that no case writes", name)
		}
	}
}

// readExamplePage returns the text of the output page.
func readExamplePage(t *testing.T) string {
	t.Helper()
	data, err := osfile.ReadFile(examplePage)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// runExample runs the command line of a case in a new directory under work,
// where out/ is its output directory, and returns what the block shows with
// work removed from the front of each path, so the archive reads as the
// page places it.
func runExample(t *testing.T, work string, test exampleCase) string {
	t.Helper()
	words, err := splitCommandLine(test.command)
	if err != nil || len(words) < 2 || words[0] != "doubleagent" {
		t.Fatalf("%q is not a doubleagent command line: %v", test.command, err)
	}
	args := words[1:]
	for i, word := range args {
		if strings.HasPrefix(word, "/cases/") {
			args[i] = filepath.Join(work, filepath.FromSlash(word))
		}
	}
	directory := filepath.Join(work, test.name)
	if err := osfile.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(directory)
	environment := goldenEnvironment()
	environment.Version = exampleVersion
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), args, environment, &stdout, &stderr); code != ExitFound {
		t.Fatalf("exit %d, want %d; stderr:\n%s", code, ExitFound, stderr.String())
	}
	text := stdout.String()
	if test.file != "" {
		data, err := osfile.ReadFile(filepath.Join(directory, "out", test.file))
		if err != nil {
			t.Fatal(err)
		}
		text = string(data)
	}
	return strings.ReplaceAll(normaliseRoot(t, text, work), "{ROOT}/", "/")
}

// splitCommandLine returns the words of a command line as sh splits one
// made of plain words separated by spaces and words quoted whole in single
// quotes, the only forms the page's command lines use.
func splitCommandLine(line string) ([]string, error) {
	var words []string
	for rest := strings.TrimSpace(line); rest != ""; rest = strings.TrimLeft(rest, " ") {
		if quoted, found := strings.CutPrefix(rest, "'"); found {
			word, after, closed := strings.Cut(quoted, "'")
			if !closed || (after != "" && after[0] != ' ') {
				return nil, errors.New("a quoted word does not end at a quote and a space")
			}
			words, rest = append(words, word), after
			continue
		}
		word, after, _ := strings.Cut(rest, " ")
		if strings.ContainsAny(word, `'"\$`) {
			return nil, errors.New("the word " + word + " needs quoting the test does not read")
		}
		words, rest = append(words, word), after
	}
	return words, nil
}
