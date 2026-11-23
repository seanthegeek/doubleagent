package analyze

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserregistry"
)

// The golden tests freeze the whole output of detect, timeline and fleet
// over inputs holding many agents. The expected files under
// testdata/golden/<case>/ are what the binary wrote; setting
// DOUBLEAGENT_UPDATE_GOLDEN=1 rewrites them and nothing else does.
// Only the working directory's path is normalised, to {ROOT}, in every
// form it prints in; normaliseRoot fails when a raw copy is left, and
// each case names the text its stdout must show. A file over
// goldenCompressAbove bytes is kept gzip-compressed and compared after
// decompression (read one with gunzip -c).

// updateGoldenVariable is the environment variable that rewrites the
// expected files.
const updateGoldenVariable = "DOUBLEAGENT_UPDATE_GOLDEN"

// goldenCompressAbove is the size in bytes above which an expected file is
// kept compressed.
const goldenCompressAbove = 32 << 10

// goldenDirectory is where the expected files of each case live.
const goldenDirectory = "testdata/golden"

// goldenCase is one command run and the files it is compared by.
type goldenCase struct {
	name    string
	command string
	// options come before the input on the command line.
	options []string
	// input is the key of goldenInputs the run reads.
	input string
	// outputs are the files the run writes under its output directory
	// that are compared besides its standard output.
	outputs []string
	// shows is the text, with {ROOT} for the working directory, that the
	// run's standard output must hold after normalising.
	shows string
}

// timelineOutputs are the files timeline writes.
var timelineOutputs = []string{TimelineFile, SessionsFile, InventoryFile}

// goldenCases returns every run frozen, in the order the report lists them.
func goldenCases() []goldenCase {
	var cases []goldenCase
	for _, input := range []string{"example", "image", "loose"} {
		shows := "{ROOT}/" + input
		if input == "example" {
			shows = "{ROOT}/example.tar.gz"
		}
		cases = append(cases,
			goldenCase{name: "detect-" + input, command: "detect", input: input, shows: shows},
			goldenCase{name: "detect-json-" + input, command: "detect", options: []string{"--json"},
				input: input, shows: "{ROOT}/"},
			goldenCase{name: "detect-files-" + input, command: "detect", options: []string{"--files"},
				input: input, shows: shows},
			goldenCase{name: "timeline-" + input, command: "timeline", input: input,
				outputs: timelineOutputs, shows: shows})
	}
	return append(cases,
		goldenCase{name: "detect-json-files-example", command: "detect",
			options: []string{"--json", "--files", "--no-secrets"}, input: "example", shows: "{ROOT}/"},
		goldenCase{name: "timeline-loose-no-secrets", command: "timeline", options: []string{"--no-secrets"},
			input: "loose", outputs: timelineOutputs, shows: "{ROOT}/loose"},
		goldenCase{name: "timeline-example-no-secrets", command: "timeline",
			options: []string{"--no-secrets", "--redaction-placeholder", "[withheld]"}, input: "example",
			outputs: timelineOutputs, shows: "{ROOT}/example.tar.gz"},
		goldenCase{name: "timeline-example-thinking", command: "timeline",
			options: []string{"--include-thinking"}, input: "example", outputs: timelineOutputs,
			shows: "{ROOT}/example.tar.gz"},
		goldenCase{name: "timeline-example-window", command: "timeline",
			options: []string{"--since", "2026-10-01", "--until", "2026-10-03"}, input: "example",
			outputs: timelineOutputs, shows: "{ROOT}/example.tar.gz"},
		goldenCase{name: "timeline-example-match", command: "timeline",
			options: []string{"--match", "README|pytest", "-i"}, input: "example",
			outputs: timelineOutputs, shows: "{ROOT}/example.tar.gz"},
		goldenCase{name: "fleet", command: "fleet", input: "fleet", outputs: []string{"fleet.csv"},
			shows: "{ROOT}/"})
}

// goldenEnvironment returns the environment of the runs: every registered
// parser, a pinned clock and a pinned version.
func goldenEnvironment() Environment {
	return Environment{
		Now:     func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parserregistry.New,
		Version: "0.0.0-golden",
	}
}

func TestGoldenOutputOfTheAnalysisCommands(t *testing.T) {
	checkNoStaleGoldenFiles(t, goldenCases())
	work := buildGoldenInputs(t)
	for _, test := range goldenCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" && test.input != "example" && test.input != "fleet" {
				t.Skip("the directory inputs are read with POSIX paths; the example archive covers Windows")
			}
			runGolden(t, work, test)
		})
	}
}

// runGolden runs one case over the shared inputs in work and compares what
// it printed and wrote with the expected files.
func runGolden(t *testing.T, work string, test goldenCase) {
	output := filepath.Join(work, "out-"+test.name)
	args := append([]string{test.command}, test.options...)
	input := filepath.Join(work, goldenInputFile(test.input))
	switch test.command {
	case "timeline":
		args = append(args, input, output)
	case "fleet":
		args = append(args, "-o", filepath.Join(output, "fleet.csv"), input)
	default:
		args = append(args, input)
	}
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), args, goldenEnvironment(), &stdout, &stderr); code != ExitFound {
		t.Fatalf("exit %d, want %d; stderr:\n%s", code, ExitFound, stderr.String())
	}
	shownOut := normaliseRoot(t, stdout.String(), work)
	if !strings.Contains(shownOut, test.shows) {
		t.Errorf("stdout does not show %q after the working directory is written {ROOT}", test.shows)
	}
	compareGolden(t, test.name, "stdout.txt", shownOut)
	compareGolden(t, test.name, "stderr.txt", normaliseRoot(t, stderr.String(), work))
	for _, name := range test.outputs {
		data, err := osfile.ReadFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		compareGolden(t, test.name, name, normaliseRoot(t, string(data), work))
	}
}

// normaliseRoot returns text with the working directory written {ROOT},
// in each form it can be printed in, and fails the test when a raw copy of
// the directory remains.
func normaliseRoot(t *testing.T, text, work string) string {
	t.Helper()
	roots := []string{work}
	if resolved, err := filepath.EvalSymlinks(work); err == nil && resolved != work {
		roots = append(roots, resolved)
	}
	for _, root := range roots {
		text = showRoot(text, root)
	}
	text = slashRoot(text)
	for _, root := range roots {
		if strings.Contains(text, root) {
			t.Errorf("the working directory %s is still in the output after normalising", root)
		}
	}
	return text
}

// compareGolden compares text with the expected file name of the case, or
// rewrites that file when DOUBLEAGENT_UPDATE_GOLDEN is 1. An empty text has
// no file, and a file that exists for an empty text is a difference.
func compareGolden(t *testing.T, name, file, got string) {
	t.Helper()
	path := filepath.Join(goldenDirectory, name, file)
	if os.Getenv(updateGoldenVariable) == "1" {
		writeGolden(t, path, got)
		return
	}
	want, found := readGolden(t, path)
	if !found && got == "" {
		return
	}
	if !found {
		t.Errorf("%s: no expected file; run with %s=1 to write it after reviewing the output",
			path, updateGoldenVariable)
		return
	}
	compareOutput(t, path, want, got)
}

// readGolden returns the text of an expected file, held plain or with a
// ".gz" suffix, and whether one exists.
func readGolden(t *testing.T, path string) (string, bool) {
	t.Helper()
	data, err := osfile.ReadFile(path)
	if err == nil {
		return string(data), true
	}
	data, err = osfile.ReadFile(path + ".gz")
	if err != nil {
		return "", false
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s.gz: %v", path, err)
	}
	text, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("%s.gz: %v", path, err)
	}
	return string(text), true
}

// writeGolden writes text as the expected file path, compressed when it is
// large, and removes the other form of the file; an empty text leaves no
// file.
func writeGolden(t *testing.T, path, text string) {
	t.Helper()
	target, other := path, path+".gz"
	if len(text) > goldenCompressAbove {
		target, other = other, target
		var buffer bytes.Buffer
		writer, _ := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
		if _, err := writer.Write([]byte(text)); err != nil || writer.Close() != nil {
			t.Fatalf("%s: cannot compress", path)
		}
		text = buffer.String()
	}
	_ = osfile.Remove(other)
	if text == "" {
		_ = osfile.Remove(target)
		return
	}
	writeFile(t, target, text)
	t.Logf("wrote %s (%d bytes)", target, len(text))
}

// checkNoStaleGoldenFiles fails the test on each stale path under
// goldenDirectory (staleGoldenPaths), so a case renamed or an output
// dropped cannot leave an expected file nothing reads.
// DOUBLEAGENT_UPDATE_GOLDEN=1 removes them instead.
func checkNoStaleGoldenFiles(t *testing.T, cases []goldenCase) {
	t.Helper()
	stale, err := staleGoldenPaths(goldenDirectory, cases)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range stale {
		staleGolden(t, path.path, path.why)
	}
}

// stalePath is an expected file or directory nothing reads, and why.
type stalePath struct {
	path string
	why  string
}

// staleGoldenPaths lists, under directory, each entry that names no case,
// each file in a case's directory that the case does not compare (its
// stdout.txt, stderr.txt and outputs, each plain or compressed), and each
// compressed file whose plain form is there too, since readGolden reads the
// plain one first.
func staleGoldenPaths(directory string, cases []goldenCase) ([]stalePath, error) {
	compared := map[string]map[string]bool{}
	for _, test := range cases {
		names := map[string]bool{}
		for _, file := range append([]string{"stdout.txt", "stderr.txt"}, test.outputs...) {
			names[file], names[file+".gz"] = true, true
		}
		compared[test.name] = names
	}
	directories, err := osfile.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var stale []stalePath
	for _, entry := range directories {
		caseDirectory := filepath.Join(directory, entry.Name())
		names, isCase := compared[entry.Name()]
		if !isCase || !entry.IsDir() {
			stale = append(stale, stalePath{caseDirectory, "names no case"})
			continue
		}
		files, err := osfile.ReadDir(caseDirectory)
		if err != nil {
			return nil, err
		}
		present := map[string]bool{}
		for _, file := range files {
			present[file.Name()] = true
		}
		for _, file := range files {
			path := filepath.Join(caseDirectory, file.Name())
			plain, compressed := strings.CutSuffix(file.Name(), ".gz")
			switch {
			case !names[file.Name()]:
				stale = append(stale, stalePath{path, "is no file its case compares"})
			case compressed && present[plain]:
				stale = append(stale, stalePath{path, "is read by nothing, since " + plain +
					" beside it is read first"})
			}
		}
	}
	return stale, nil
}

// staleGolden removes path when the expected files are being rewritten and
// otherwise fails the test, saying why the path is stale.
func staleGolden(t *testing.T, path, why string) {
	t.Helper()
	if os.Getenv(updateGoldenVariable) == "1" {
		if err := osfile.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Errorf("%s %s; remove it or add it to a case", path, why)
}
