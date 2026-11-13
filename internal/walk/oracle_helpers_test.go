package walk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// shRun is what one sh collector run left: its manifest lines, its log lines
// without their times, and its kept staging directory.
type shRun struct {
	rows  []string
	log   []string
	stage string
}

// runShCollector runs the sh collector under dash in image mode over root with
// the staging directory kept.
func runShCollector(t *testing.T, root string, options []string) shRun {
	t.Helper()
	outDir := t.TempDir()
	args := append([]string{"../../collectors/collect-agent-artifacts.sh", "-r", root, "-o",
		outDir, "-q", "-k", "--no-docker", "--no-projects"}, options...)
	command := exec.Command("dash", args...)
	command.Env = append(os.Environ(), "COLLECTOR_SH=dash")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sh collector: %v\n%s", err, output)
	}
	run := shRun{rows: readLines(t, outDir, "*.manifest.jsonl")}
	for _, line := range readLines(t, outDir, "*.log") {
		run.log = append(run.log, logTime.ReplaceAllString(line, ""))
	}
	stages, _ := filepath.Glob(outDir + "/.stage-*")
	if len(stages) != 1 {
		t.Fatalf("staging directories %v", stages)
	}
	run.stage = stages[0]
	return run
}

// readLines returns the lines of the one file in dir matching pattern.
func readLines(t *testing.T, dir, pattern string) []string {
	t.Helper()
	matches, _ := filepath.Glob(dir + "/" + pattern)
	if len(matches) != 1 {
		t.Fatalf("%s files: %v", pattern, matches)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// lineLog keeps log lines in memory.
type lineLog struct{ lines []string }

// Line keeps text.
func (log *lineLog) Line(text string) error {
	log.lines = append(log.lines, text)
	return nil
}

// runSettings are the Go collector settings that match one sh option list.
type runSettings struct {
	full      bool
	noSecrets bool
	maxSize   int64
}

// oracleRuns are the option lists each oracle comparison runs both
// collectors with.
var oracleRuns = []struct {
	name     string
	options  []string
	settings runSettings
}{
	{"a default run", nil, runSettings{maxSize: 256 << 20}},
	{"a run without secrets and a 1 MB cap", []string{"--no-secrets", "--max-file-size", "1"},
		runSettings{noSecrets: true, maxSize: 1 << 20}},
	{"a full run", []string{"--full"}, runSettings{full: true, maxSize: 256 << 20}},
}

// runGoCollector collects the homes of root as the sh run did and returns
// the rows, the log lines and the staging directory.
func runGoCollector(t *testing.T, root string, settings runSettings) shRun {
	t.Helper()
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	stageDir := t.TempDir() + "/stage"
	if err := os.MkdirAll(stageDir+"/fs", 0o700); err != nil {
		t.Fatal(err)
	}
	physical, err := stage.PhysicalDir(stageDir)
	if err != nil {
		t.Fatal(err)
	}
	var rows bytes.Buffer
	log := &lineLog{}
	collector := &Collector{
		Tables: tables, Excludes: tables.Excludes, Secret: MatchAny(tables.SecretGlobs),
		Log: log, Manifest: manifest.NewWriter(&rows), Root: root, MaxSize: settings.maxSize,
		NoSecrets: settings.noSecrets,
		Stager:    stage.Stager{Dir: stageDir, Physical: physical, Root: root},
	}
	if settings.full {
		collector.Excludes = nil
	}
	homes := users.Enumerate(false, root, "", nil)
	if err := collector.CollectUserHomes(context.Background(), homes); err != nil {
		t.Fatal(err)
	}
	return shRun{rows: strings.Split(strings.TrimSuffix(rows.String(), "\n"), "\n"),
		log: log.lines, stage: stageDir}
}

// volatileFields matches the row fields the comparison drops: atime, which
// the sh run's own reads change, and the error text, a documented difference.
var volatileFields = regexp.MustCompile(`"atime":[0-9]+,|"error":"(\\.|[^"\\])*"`)

// walkLine matches the log lines the home walk writes.
var walkLine = regexp.MustCompile(`^(User |  \[|read failed: |copy failed: |find: )`)

// logTime matches the time a log line starts with; find's own lines have none.
var logTime = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z `)

// logReason matches the reason ending a failure line, the error text the
// comparison drops: the fixture paths hold no ": ".
var logReason = regexp.MustCompile(`^((?:find|read failed|copy failed): [^:]*): .*?` +
	`(; staged path resolves outside the staging directory, not removed)?$`)

// compareRows checks that the sh manifest starts with the Go rows, fields
// dropped, and that every sh row after them belongs to the relocation step.
func compareRows(t *testing.T, want, got []string) {
	t.Helper()
	if len(got) > len(want) {
		t.Fatalf("%d Go rows, %d sh rows", len(got), len(want))
	}
	for i, row := range got {
		if volatileFields.ReplaceAllString(row, "") != volatileFields.ReplaceAllString(want[i], "") {
			t.Fatalf("row %d:\n sh %s\n go %s", i, want[i], row)
		}
	}
	for _, row := range want[len(got):] {
		if !strings.Contains(row, `"agent":"shared"`) {
			t.Fatalf("sh row past the home rows is not the relocation step's: %s", row)
		}
	}
	t.Logf("%d home rows equal", len(got))
}

// compareLog checks that the sh log, after its two start lines, holds the
// Go walk's lines next and no other walk line after them.
func compareLog(t *testing.T, want, got []string) {
	t.Helper()
	want = want[2:]
	if len(got) > len(want) {
		t.Fatalf("%d Go log lines, %d sh lines", len(got), len(want))
	}
	for i, line := range got {
		if logReason.ReplaceAllString(line, "$1$2") != logReason.ReplaceAllString(want[i], "$1$2") {
			t.Fatalf("log line %d:\n sh %q\n go %q", i, want[i], line)
		}
	}
	if len(want) > len(got) && walkLine.MatchString(want[len(got)]) {
		t.Fatalf("sh walk line past the Go lines: %q", want[len(got)])
	}
}

// compareStage checks that every entry the Go run staged is in the sh
// staging directory with the same type, content or link target.
func compareStage(t *testing.T, want, got string) {
	t.Helper()
	count := 0
	err := filepath.WalkDir(got, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(path, got)
		if describe(t, want+relative) != describe(t, path) {
			t.Errorf("staged %s: sh %q, go %q", relative, describe(t, want+relative),
				describe(t, path))
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d staged entries equal", count)
}

// describe returns a staged entry's type and its content hash or link target.
func describe(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Lstat(path)
	switch {
	case err != nil:
		return "missing"
	case info.Mode()&fs.ModeSymlink != 0:
		target, _ := os.Readlink(path)
		return "link " + target
	case info.IsDir():
		return "dir"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "file " + hex.EncodeToString(sum[:])
}
