//go:build unix

package walk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
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
		Stager:    stage.Stager{Dir: stageDir, Physical: physical, Root: root, Places: stage.NewPlaces()},
	}
	if settings.full {
		collector.Excludes = nil
	}
	homes := users.Enumerate(context.Background(), false, root, "", nil, nil, nil,
		NewRootCheck(root).Reached)
	if err := collector.CollectUserHomes(context.Background(), homes); err != nil {
		t.Fatal(err)
	}
	return shRun{rows: strings.Split(strings.TrimSuffix(rows.String(), "\n"), "\n"),
		log: log.lines, stage: stageDir}
}

// volatileFields matches the row fields the comparison drops: atime, which
// the sh run's own reads change, and the error text, a documented difference.
var volatileFields = regexp.MustCompile(`"atime":[0-9]+,|"error":"(\\.|[^"\\])*"`)

// archivePathField matches a row's archive_path field.
var archivePathField = regexp.MustCompile(`"archive_path":"(\\.|[^"\\])*"`)

// requireStagedMembers fails unless every row with an archive path names an
// entry of the staging tree spelled exactly so, component by component as
// each directory lists it: a collected row's file with the bytes it hashed,
// a link row's link with its target.
func requireStagedMembers(t *testing.T, stageDir string, rows []string) {
	t.Helper()
	for _, line := range rows {
		var row struct {
			ArchivePath string `json:"archive_path"`
			Status      string `json:"status"`
			SHA256      string `json:"sha256"`
			Target      string `json:"target"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil || row.ArchivePath == "" {
			continue
		}
		directory := stageDir
		for _, component := range strings.Split(row.ArchivePath, "/") {
			names, err := os.ReadDir(directory)
			if err != nil || !slices.ContainsFunc(names, func(entry fs.DirEntry) bool {
				return entry.Name() == component
			}) {
				t.Errorf("row names %s, which the staging tree does not hold under that spelling",
					row.ArchivePath)
				break
			}
			directory += "/" + component
		}
		member := stageDir + "/" + row.ArchivePath
		switch row.Status {
		case "symlink":
			if target, err := os.Readlink(member); err != nil || target != row.Target {
				t.Errorf("row names link %s, staged as %q, %v; want %q", row.ArchivePath, target,
					err, row.Target)
			}
		case "collected":
			content, err := os.ReadFile(member)
			sum := sha256.Sum256(content)
			if err != nil || hex.EncodeToString(sum[:]) != row.SHA256 {
				t.Errorf("row names %s, whose staged bytes do not hash to %s (%v)",
					row.ArchivePath, row.SHA256, err)
			}
		}
	}
}

// walkLine matches the log lines the home walk writes.
var walkLine = regexp.MustCompile(`^(User |  \[|read failed: |copy failed: |find: )`)

// logTime matches the time a log line starts with; find's own lines have none.
var logTime = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z `)

// logReason matches the reason ending a failure line, the error text the
// comparison drops: the fixture paths hold no ": ".
var logReason = regexp.MustCompile(`^((?:find|read failed|copy failed): [^:]*): .*?` +
	`(; staged path resolves outside the staging directory, not removed)?$`)

// findQuotes matches the single quotes GNU find puts around the path in its
// error lines; BSD find and the Go walk write the path bare.
var findQuotes = regexp.MustCompile(`^find: '(.*)': `)

// normaliseFailure makes a failure line comparable: the quotes GNU find adds
// go, then the reason text, which differs by platform and program.
func normaliseFailure(line string) string {
	line = findQuotes.ReplaceAllString(line, "find: $1: ")
	return logReason.ReplaceAllString(line, "$1$2")
}

// compareRows checks that the sh manifest starts with the Go rows, fields
// dropped, and that every sh row after them belongs to the relocation step.
func compareRows(t *testing.T, want, got []string, goStage string) {
	t.Helper()
	if len(got) > len(want) {
		t.Fatalf("%d Go rows, %d sh rows", len(got), len(want))
	}
	requireStagedMembers(t, goStage, got)
	// On a staging volume that takes two spellings for one, the Go row names
	// the member as the staging tree spells it (Devin/User/... staged in the
	// directory devin made), the sh row the source's spelling: only that
	// field is then compared without regard to ASCII case.
	normal := func(row string) string { return volatileFields.ReplaceAllString(row, "") }
	if stage.FoldsCase(goStage + "/fs") {
		normal = func(row string) string {
			return archivePathField.ReplaceAllStringFunc(volatileFields.ReplaceAllString(row, ""),
				strings.ToLower)
		}
	}
	for i, row := range got {
		if normal(row) != normal(want[i]) {
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
		if normaliseFailure(line) != normaliseFailure(want[i]) {
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
