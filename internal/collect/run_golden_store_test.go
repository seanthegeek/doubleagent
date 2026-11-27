package collect_test

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// updateGoldenVariable is the environment variable that rewrites the
// expected files.
const updateGoldenVariable = "DOUBLEAGENT_UPDATE_GOLDEN"

// goldenRoot holds the expected files, in <directory>/<case>/, one file per
// output (the log as log.txt, since git ignores *.log): windows/ on
// Windows; on Linux and macOS unix/, which both read, with linux/<case>/ or
// darwin/<case>/ holding only how that platform's output differs from
// unix/'s (run_golden_overlay_test.go). A file of a case over the main image
// may be a delta from the default case's file of the same directory
// (name.delta).
const goldenRoot = "testdata/golden"

// sharedDirectory is the directory Linux and macOS both read.
const sharedDirectory = "unix"

// deltaSuffix ends the name of a file kept as a delta from the default case.
const deltaSuffix = ".delta"

// familyDirectory names the directory of the whole cases this platform
// reads: unix/ on Linux and macOS, windows/ on Windows, its own elsewhere.
func familyDirectory() string {
	if otherUnixPlatform() != "" {
		return sharedDirectory
	}
	return runtime.GOOS
}

// updating reports whether the run rewrites the expected files.
func updating() bool {
	return os.Getenv(updateGoldenVariable) == "1"
}

// caseFiles is the stored form of one case: file name to content, with a
// delta's name ending in deltaSuffix.
type caseFiles map[string]string

// readCase returns the files of a case's directory, and false when the
// directory does not exist.
func readCase(test testing.TB, directory string) (caseFiles, bool) {
	test.Helper()
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false
	}
	if err != nil {
		test.Fatal(err)
	}
	files := caseFiles{}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			test.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	return files, true
}

// writeCase replaces a case's directory with the files.
func writeCase(test testing.TB, directory string, files caseFiles) {
	test.Helper()
	if err := os.RemoveAll(directory); err != nil {
		test.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		test.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			test.Fatal(err)
		}
	}
}

// removeCase removes a case's directory.
func removeCase(test testing.TB, directory string) {
	test.Helper()
	if err := os.RemoveAll(directory); err != nil {
		test.Fatal(err)
	}
}

// caseDirectory returns the directory of a case under one platform
// directory.
func caseDirectory(platform, name string) string {
	return filepath.Join(goldenRoot, platform, name)
}

// expandCase returns a case's stored files with each delta applied to the
// default case's file of the same name.
func expandCase(test testing.TB, files, defaults map[string]string) map[string]string {
	test.Helper()
	expanded := map[string]string{}
	for name, content := range files {
		plain, isDelta := strings.CutSuffix(name, deltaSuffix)
		if !isDelta {
			expanded[name] = content
			continue
		}
		text, err := applyDelta(defaults[plain], content)
		if err != nil {
			test.Errorf("%s: %v", name, err)
		}
		expanded[plain] = text
	}
	return expanded
}

// storedForm returns the files a case stores for its outputs: each as it
// is, or, for a case over the main image, as a delta from the default
// case's file when that is shorter and applies back to the output.
func storedForm(outputs, defaults map[string]string, overMainImage bool) caseFiles {
	stored := caseFiles{}
	for name, text := range outputs {
		base, hasBase := defaults[name]
		if overMainImage && hasBase {
			delta := makeDelta(base, text)
			if applied, err := applyDelta(base, delta); err == nil && applied == text && len(delta) < len(text) {
				stored[name+deltaSuffix] = delta
				continue
			}
		}
		stored[name] = text
	}
	return stored
}

// differingLines returns the first limit removed and added lines from want
// to got, each after its "-" or "+".
func differingLines(want, got string, limit int) string {
	var lines []string
	for _, step := range diffLines(splitLines(want), splitLines(got)) {
		if step.kind != ' ' {
			lines = append(lines, string(step.kind)+step.line)
		}
	}
	if len(lines) > limit {
		lines = append(lines[:limit], "... and more")
	}
	return strings.Join(lines, "\n")
}

// firstLines returns the first limit lines of text.
func firstLines(text string, limit int) string {
	lines := splitLines(text)
	if len(lines) > limit {
		lines = append(lines[:limit], "...")
	}
	return strings.Join(lines, "\n")
}

// compareCase reports every file of the case whose expected text differs
// from the run's, with the lines that differ.
func compareCase(test testing.TB, want, got map[string]string) {
	test.Helper()
	names := slices.Sorted(maps.Keys(want))
	for name := range got {
		if _, found := want[name]; !found {
			names = append(names, name)
		}
	}
	for _, name := range names {
		expected, inWant := want[name]
		actual, inGot := got[name]
		switch {
		case !inGot:
			test.Errorf("%s: expected, and the run wrote none", name)
		case !inWant:
			test.Errorf("%s: the run wrote one, and none is expected:\n%s", name, firstLines(actual, 20))
		case expected != actual:
			test.Errorf("%s differs from the expected file (- expected, + the run's):\n%s", name,
				differingLines(expected, actual, 40))
		}
	}
}
