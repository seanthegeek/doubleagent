package collect

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runName is the name every output of a test run starts with: the host and
// the start time testHost pins.
func runName() string {
	return safeHostname(hostname() + "_20260102T030405Z_agent-artifacts")
}

// lastLogLine returns the last line of the run's log in outDir.
func lastLogLine(t *testing.T, outDir string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(runOutput(t, outDir, ".log"), "\n"), "\n")
	return lines[len(lines)-1]
}

// plantLink makes a symbolic link at path to target, skipping the test where
// the account cannot make one.
func plantLink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
}

func TestThePartialArchiveIsARegularFileTheRunCreatesNew(t *testing.T) {
	name := runName()
	partialName, finalName := name+".tar.gz.partial", name+".tar.gz"
	cases := []struct {
		name string
		// plant puts an entry at the partial or final name before the run.
		plant    func(t *testing.T, outDir, outside string)
		wantCode int
		// keptEntry is the planted name that must still be there.
		keptEntry string
	}{
		{"a link at the partial name is removed and never written through",
			func(t *testing.T, outDir, outside string) {
				plantLink(t, outside, filepath.Join(outDir, partialName))
			}, exitOK, ""},
		{"a link at the final name is replaced and never written through",
			func(t *testing.T, outDir, outside string) {
				plantLink(t, outside, filepath.Join(outDir, finalName))
			}, exitOK, ""},
		{"an empty directory at the partial name is removed",
			func(t *testing.T, outDir, _ string) {
				if err := os.Mkdir(filepath.Join(outDir, partialName), 0o700); err != nil {
					t.Fatal(err)
				}
			}, exitOK, ""},
		{"a directory holding a file at the partial name ends the run, is left as it was, and an" +
			" earlier file at the final name is gone",
			func(t *testing.T, outDir, _ string) {
				if err := os.WriteFile(filepath.Join(outDir, finalName), nil, 0o600); err != nil {
					t.Fatal(err)
				}
				dir := filepath.Join(outDir, partialName)
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "kept"), []byte("kept\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}, exitFatal, partialName + "/kept"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			outDir := longTempDir(t)
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			testCase.plant(t, outDir, outside)
			var stdout, stderr bytes.Buffer
			code := runCollection(context.Background(), testOptions(fixtureImage(t), outDir), testHost(t),
				&stdout, &stderr)
			if code != testCase.wantCode {
				t.Fatalf("exit code %d, want %d; log ends %q", code, testCase.wantCode, lastLogLine(t, outDir))
			}
			if text, err := os.ReadFile(outside); err != nil || string(text) != "outside\n" {
				t.Errorf("the file outside the output directory holds %d bytes (%v), want it untouched", len(text), err)
			}
			checkPartialOutcome(t, outDir, testCase.wantCode, testCase.keptEntry)
		})
	}
}

// checkPartialOutcome fails the test unless a run that exited code left a
// regular file at the archive's final name and nothing at its partial name
// (code 0), or nothing at the final name, no sidecar, the planted entry kept
// and a FATAL line naming the partial path (code 2).
func checkPartialOutcome(t *testing.T, outDir string, code int, keptEntry string) {
	t.Helper()
	name := runName()
	partial, final := filepath.Join(outDir, name+".tar.gz.partial"), filepath.Join(outDir, name+".tar.gz")
	info, finalErr := os.Lstat(final)
	if code == exitOK {
		if finalErr != nil || !info.Mode().IsRegular() {
			t.Errorf("the final name holds %v (%v), want a regular file", info, finalErr)
		}
		if _, err := os.Lstat(partial); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the partial name is still taken: %v", err)
		}
		return
	}
	if !errors.Is(finalErr, os.ErrNotExist) {
		t.Errorf("the failed run left %v at the final name", info)
	}
	if _, err := os.Lstat(final + ".sha256"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the failed run left a sidecar: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(outDir, keptEntry)); err != nil {
		t.Errorf("the planted %s is gone: %v", keptEntry, err)
	}
	last := lastLogLine(t, outDir)
	if !strings.Contains(last, " FATAL: archive not written: ") || !strings.Contains(last, partial) {
		t.Errorf("the log ends %q, want a FATAL line naming %s", last, partial)
	}
}
