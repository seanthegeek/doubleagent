package collect

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// outsideFiles is a file and a directory outside the output directory, as
// another account would aim a planted link at them, and the path of one
// that does not exist yet, for a link that dangles.
type outsideFiles struct {
	file, directory, missing string
}

// newOutsideFiles makes the outside file, holding a known line, and the
// empty outside directory.
func newOutsideFiles(t *testing.T) outsideFiles {
	t.Helper()
	parent := t.TempDir()
	outside := outsideFiles{file: filepath.Join(parent, "file"), directory: filepath.Join(parent, "dir"),
		missing: filepath.Join(parent, "missing")}
	if err := os.WriteFile(outside.file, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside.directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return outside
}

// checkUntouched fails the test unless the outside file still holds its
// line, the outside directory is still empty and the missing path is still
// missing.
func (outside outsideFiles) checkUntouched(t *testing.T) {
	t.Helper()
	if text, err := os.ReadFile(outside.file); err != nil || string(text) != "outside\n" {
		t.Errorf("the file outside the output directory holds %q (%v), want it untouched", text, err)
	}
	if entries, err := os.ReadDir(outside.directory); err != nil || len(entries) != 0 {
		t.Errorf("the directory outside the output directory holds %d entries (%v), want none",
			len(entries), err)
	}
	if _, err := os.Lstat(outside.missing); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the run created %s through a dangling link: %v", outside.missing, err)
	}
}

func TestEveryOutputIsCreatedNewAndNeverThroughALinkAtItsName(t *testing.T) {
	name := runName()
	cases := []struct {
		name, entry string
		// target picks what the planted link points at.
		target func(outsideFiles) string
		// wantIn is where the refusal is reported: "stderr" or "log".
		wantIn string
	}{
		{"a link at the log's name ends the run before it starts", name + ".log",
			func(outside outsideFiles) string { return outside.file }, "stderr"},
		{"a dangling link at the log's name creates nothing behind it", name + ".log",
			func(outside outsideFiles) string { return outside.missing }, "stderr"},
		{"a link at the manifest's name ends the run before it starts", name + ".manifest.jsonl",
			func(outside outsideFiles) string { return outside.file }, "stderr"},
		{"a dangling link at the manifest's name creates nothing behind it", name + ".manifest.jsonl",
			func(outside outsideFiles) string { return outside.missing }, "stderr"},
		{"a link at the staging directory's name ends the run before it starts", ".stage-" + name,
			func(outside outsideFiles) string { return outside.directory }, "stderr"},
		{"a link at collection.json's name ends the run before the archive", name + ".collection.json",
			func(outside outsideFiles) string { return outside.file }, "log"},
		{"a dangling link at collection.json's name creates nothing behind it",
			name + ".collection.json", func(outside outsideFiles) string { return outside.missing }, "log"},
		{"a link at the sidecar's name ends the run before the archive has its name",
			name + ".tar.gz.sha256", func(outside outsideFiles) string { return outside.file }, "log"},
		{"a dangling link at the sidecar's name creates nothing behind it", name + ".tar.gz.sha256",
			func(outside outsideFiles) string { return outside.missing }, "log"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			outDir := longTempDir(t)
			outside := newOutsideFiles(t)
			planted := filepath.Join(outDir, testCase.entry)
			plantLink(t, testCase.target(outside), planted)
			var stdout, stderr bytes.Buffer
			code := runCollection(context.Background(), testOptions(fixtureImage(t), outDir), testHost(t),
				&stdout, &stderr)
			outside.checkUntouched(t)
			if code != exitFatal {
				t.Fatalf("exit code %d, want %d; stderr %q", code, exitFatal, stderr.String())
			}
			if info, err := os.Lstat(planted); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("the planted link is gone or changed: %v, %v", info, err)
			}
			if _, err := os.Lstat(filepath.Join(outDir, name+".tar.gz")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the refused run left an archive: %v", err)
			}
			report := stderr.String()
			if testCase.wantIn == "log" {
				report = lastLogLine(t, outDir)
			}
			if !strings.Contains(report, syspath.ToNative(planted)) {
				t.Errorf("the %s says %q, want it to name %s", testCase.wantIn, report, planted)
			}
		})
	}
}

func TestAPartialArchiveSwappedDuringTheRunIsNeitherHashedNorMoved(t *testing.T) {
	name := runName()
	outDir := longTempDir(t)
	outside := newOutsideFiles(t)
	partial := filepath.Join(outDir, name+".tar.gz.partial")
	host := testHost(t)
	host.reached = func(point string) {
		if point != archiveWritten {
			return
		}
		if err := os.Remove(partial); err != nil {
			t.Fatal(err)
		}
		plantLink(t, outside.file, partial)
	}
	var stdout, stderr bytes.Buffer
	code := runCollection(context.Background(), testOptions(fixtureImage(t), outDir), host, &stdout,
		&stderr)
	outside.checkUntouched(t)
	if code != exitFatal {
		t.Fatalf("exit code %d, want %d; log ends %q", code, exitFatal, lastLogLine(t, outDir))
	}
	for _, left := range []string{name + ".tar.gz", name + ".tar.gz.sha256"} {
		if _, err := os.Lstat(filepath.Join(outDir, left)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the run left %s: %v", left, err)
		}
	}
	if info, err := os.Lstat(partial); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the planted link at the partial name is gone or changed: %v, %v", info, err)
	}
	last := lastLogLine(t, outDir)
	if !strings.Contains(last, " FATAL: archive not written: ") || !strings.Contains(last, partial) {
		t.Errorf("the log ends %q, want a FATAL line naming %s", last, partial)
	}
}
