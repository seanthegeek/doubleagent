package analyze

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestAnOutputInsideADirectoryInputIsRefused(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "loose")
	writeFile(t, filepath.Join(tree, "home", "a", ".claude", "history.jsonl"), "{}\n")
	archive := filepath.Join(dir, "home.tar")
	writeHomeArchive(t, archive)
	inventories := filepath.Join(dir, "inventories")
	writeFile(t, filepath.Join(inventories, "h.jsonl"), `{"type":"host","host":"h"}`+"\n")
	cases := []struct {
		name    string
		args    []string
		code    int
		message string
	}{
		{"timeline into the directory it reads", []string{"timeline", tree,
			filepath.Join(tree, "home", "a", ".claude", "out")}, ExitError,
			"the output directory " + filepath.Join(tree, "home", "a", ".claude", "out") + " must be outside the input"},
		{"timeline into the input itself", []string{"timeline", tree, tree}, ExitError,
			"is not empty"},
		{"fleet into the directory it reads", []string{"fleet", "-o", filepath.Join(inventories, "f.csv"),
			inventories}, ExitError, "must be outside the input directory " + inventories},
		{"timeline beside an archive it reads", []string{"timeline", archive, filepath.Join(dir, "out")},
			ExitFound, ""},
		{"timeline into a sibling of the input", []string{"timeline", tree, tree + "-out"}, ExitFound, ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), test.args, System(), &stdout, &stderr)
			if code != test.code || !strings.Contains(stderr.String(), test.message) {
				t.Fatalf("exit %d, stderr %q", code, stderr.String())
			}
			if test.code == ExitError && !strings.Contains(stderr.String(), "\nUsage: doubleagent ") {
				t.Errorf("not a usage error: %q", stderr.String())
			}
		})
	}
	for _, unwritten := range []string{filepath.Join(tree, "home", "a", ".claude", "out"),
		filepath.Join(tree, timelineFile), filepath.Join(inventories, "f.csv")} {
		if _, err := os.Lstat(unwritten); err == nil {
			t.Errorf("%s was written into the input", unwritten)
		}
	}
}

// deepTree makes directories d/d/... below root, one more level each until
// the system refuses a longer path or 2000 levels stand, and returns the
// one halfway down: its path leaves room for the files written in it, and
// text that climbs from it to the file system's root by adding /.. per
// level passes the longest path the system takes.
func deepTree(t *testing.T, root string) string {
	t.Helper()
	var levels []string
	for current := root; len(levels) < 2000; {
		current = filepath.Join(current, "d")
		if err := osfile.MkdirAll(current, 0o700); err != nil {
			break
		}
		levels = append(levels, current)
	}
	if len(levels) < 100 {
		t.Fatalf("only %d levels could be made", len(levels))
	}
	return levels[len(levels)/2]
}

func TestAnOutputDeepInsideADirectoryInputIsRefused(t *testing.T) {
	tree := filepath.Join(t.TempDir(), "loose")
	writeFile(t, filepath.Join(tree, "home", "a", ".claude", "history.jsonl"), "{}\n")
	parent := deepTree(t, tree)
	deep, csv := filepath.Join(parent, "out"), filepath.Join(parent, "f.csv")
	for _, args := range [][]string{{"timeline", tree, deep}, {"fleet", "-o", csv, tree}} {
		code, _, stderr := runCommand(testEnvironment(), args...)
		if code != ExitError || !strings.Contains(stderr, "must be outside the input") {
			t.Errorf("%s exited %d: %q", args[0], code, stderr)
		}
	}
	for _, unwritten := range []string{deep, csv} {
		if lstatExists(unwritten) {
			t.Errorf("%s was written into the input", unwritten)
		}
	}
}

func TestADeepOutputOutsideTheInputIsWritten(t *testing.T) {
	work := t.TempDir()
	tree := filepath.Join(work, "loose")
	writeFile(t, filepath.Join(tree, "home", "a", ".claude", "history.jsonl"), "{}\n")
	inventories := filepath.Join(work, "inventories")
	writeFile(t, filepath.Join(inventories, "h.jsonl"), hostLine)
	parent := deepTree(t, filepath.Join(work, "elsewhere"))
	relative, err := filepath.Rel(work, parent)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, output, written string
		args                  []string
	}{
		{"timeline, absolute", filepath.Join(parent, "out1"), filepath.Join(parent, "out1", timelineFile),
			[]string{"timeline", tree}},
		{"timeline, relative", filepath.Join(relative, "out2"), filepath.Join(parent, "out2", timelineFile),
			[]string{"timeline", tree}},
		{"fleet, absolute", filepath.Join(parent, "f1.csv"), filepath.Join(parent, "f1.csv"),
			[]string{"fleet", inventories}},
		{"fleet, relative", filepath.Join(relative, "f2.csv"), filepath.Join(parent, "f2.csv"),
			[]string{"fleet", inventories}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(work)
			args := []string{test.args[0], test.args[1], test.output}
			if test.args[0] == "fleet" {
				args = []string{"fleet", "-o", test.output, test.args[1]}
			}
			code, _, stderr := runCommand(testEnvironment(), args...)
			if code == ExitError || !lstatExists(test.written) {
				t.Errorf("exit %d, %s written: %v, stderr %q", code, test.written, lstatExists(test.written),
					stderr)
			}
		})
	}
}

func TestAnOutputBesideTheInputIsJudgedByWholeNames(t *testing.T) {
	work := t.TempDir()
	input := filepath.Join(work, "case")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	writeFile(t, filepath.Join(work, "case2", "keep"), "")
	tests := []struct {
		name, written string
		args          []string
	}{
		{"timeline into a directory whose name starts with the input's",
			filepath.Join(work, "case2", "out", timelineFile),
			[]string{"timeline", input, filepath.Join(work, "case2", "out")}},
		{"timeline into a directory not made yet, its parents missing too",
			filepath.Join(work, "new", "a", "b", "out", timelineFile),
			[]string{"timeline", input, filepath.Join(work, "new", "a", "b", "out")}},
		{"fleet into a directory whose name starts with the input's", filepath.Join(work, "case2", "f.csv"),
			[]string{"fleet", "-o", filepath.Join(work, "case2", "f.csv"), input}},
		{"fleet into a directory not made yet, its parents missing too",
			filepath.Join(work, "new2", "a", "b", "f.csv"),
			[]string{"fleet", "-o", filepath.Join(work, "new2", "a", "b", "f.csv"), input}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), test.args...)
			if code == ExitError || !lstatExists(test.written) {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
		})
	}
}

func TestAnOutputBelowAFileFailsAsNotADirectory(t *testing.T) {
	work := t.TempDir()
	input := filepath.Join(work, "inv")
	writeFile(t, filepath.Join(input, "h.jsonl"), hostLine)
	file := filepath.Join(work, "file.tar.gz")
	writeFile(t, file, "x")
	for name, args := range map[string][]string{
		"timeline": {"timeline", input, filepath.Join(file, "out")},
		"fleet":    {"fleet", "-o", filepath.Join(file, "out.csv"), input},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), args...)
			if code != ExitError || !strings.HasPrefix(stderr, "error: "+file) ||
				!strings.HasSuffix(stderr, ": "+belowFileReason+"\n") {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
		})
	}
}

func TestAnOutputInAnotherCaseOfTheInputIsRefusedWhereCaseIsIgnored(t *testing.T) {
	work := t.TempDir()
	input := filepath.Join(work, "case")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	if !sameFile(input, filepath.Join(work, "CASE")) {
		t.Skip("this file system tells letter case apart")
	}
	for name, args := range map[string][]string{
		"timeline": {"timeline", input, filepath.Join(work, "CASE", "out")},
		"fleet":    {"fleet", "-o", filepath.Join(work, "CASE", "f.csv"), input},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), args...)
			if code != ExitError || !strings.Contains(stderr, "must be outside the input") {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
		})
	}
	for _, unwritten := range []string{filepath.Join(input, "out"), filepath.Join(input, "f.csv")} {
		if lstatExists(unwritten) {
			t.Errorf("%s was written into the input", unwritten)
		}
	}
}
