package collect

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/syspath"
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

func TestAStopRequestEndsTheRunOnlyBeforeItsArchiveHasItsName(t *testing.T) {
	name := runName()
	interrupted := []string{name + ".log", name + ".manifest.jsonl"}
	finished := []string{name + ".collection.json", name + ".log", name + ".manifest.jsonl",
		name + ".tar.gz", name + ".tar.gz.sha256"}
	cases := []struct {
		name string
		// stopText stops the run when standard error is given it; stopPoint
		// when the archive step reaches it.
		stopText, stopPoint string
		wantCode            int
		wantNames           []string
		wantLast            string
	}{
		{"a stop request as archiving starts leaves only the log and the manifest",
			"Archiving\n", "", exitInterrupted, interrupted, " interrupted"},
		{"a stop request while the archive is hashed leaves only the log and the manifest",
			"", archiveWritten, exitInterrupted, interrupted, " interrupted"},
		{"a stop request after the sidecar is written, before the move, leaves only the log and the manifest",
			"", sidecarWritten, exitInterrupted, interrupted, " interrupted"},
		{"a stop request once the archive has its name changes nothing",
			"", archiveMoved, exitOK, finished, " done: "},
		{"a stop request after the done line changes nothing", "done\n", "", exitOK, finished, " done: "},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			outDir := t.TempDir()
			options := testOptions(fixtureImage(t), outDir)
			options.Quiet = false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			host := testHost(t)
			host.reached = func(point string) {
				if point == testCase.stopPoint {
					cancel()
				}
			}
			stderr := &cancelOnText{text: testCase.stopText, cancel: func() {}}
			if testCase.stopText != "" {
				stderr.cancel = cancel
			}
			var stdout bytes.Buffer
			if code := runCollection(ctx, options, host, &stdout, stderr); code != testCase.wantCode {
				t.Fatalf("exit code %d, want %d", code, testCase.wantCode)
			}
			if ctx.Err() == nil {
				t.Fatalf("the run never reached %q%q, so it was not stopped", testCase.stopText, testCase.stopPoint)
			}
			checkStoppedRun(t, outDir, testCase.wantNames, testCase.wantLast)
		})
	}
}

// checkStoppedRun fails the test unless outDir holds wantNames, the log's
// last line holds wantLast, and the log has no FATAL line and not both the
// done line and the interrupted line.
func checkStoppedRun(t *testing.T, outDir string, wantNames []string, wantLast string) {
	t.Helper()
	if got := outputNames(t, outDir); strings.Join(got, " ") != strings.Join(wantNames, " ") {
		t.Errorf("outputs %v, want %v", got, wantNames)
	}
	log := runOutput(t, outDir, ".log")
	if last := lastLogLine(t, outDir); !strings.Contains(last, wantLast) {
		t.Errorf("the log ends %q, want %q in it", last, wantLast)
	}
	if strings.Contains(log, " done: ") && strings.Contains(log, " interrupted\n") {
		t.Errorf("the log has both a done line and the interrupted line:\n%s", log)
	}
	if strings.Contains(log, "FATAL") {
		t.Errorf("a stopped run logged a FATAL line:\n%s", log)
	}
}

func TestAnArchiveThatCannotBeMovedIsKeptCompleteUnderItsPartialName(t *testing.T) {
	cases := []struct {
		name string
		// failures is how many moves fail before the real one is made.
		failures     int
		wantCode     int
		wantAttempts int
	}{
		{"a move that keeps failing leaves the archive and its sidecar under the partial name",
			100, exitFatal, moveAttempts},
		{"a move that fails twice is tried again and the run finishes", 2, exitOK, 3},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			outDir := longTempDir(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			host := testHost(t)
			attempts := 0
			var pauses []time.Duration
			host.sleep = func(pause time.Duration) { pauses = append(pauses, pause) }
			host.rename = func(oldPath, newPath string) error {
				attempts++
				// A stop request during the move changes nothing either.
				cancel()
				if attempts <= testCase.failures {
					return &os.LinkError{Op: "rename", Old: syspath.ToNative(oldPath),
						New: syspath.ToNative(newPath), Err: errors.New("in use")}
				}
				return os.Rename(oldPath, newPath)
			}
			var stdout, stderr bytes.Buffer
			code := runCollection(ctx, testOptions(fixtureImage(t), outDir), host, &stdout, &stderr)
			if code != testCase.wantCode || attempts != testCase.wantAttempts {
				t.Fatalf("exit code %d after %d moves, want %d after %d; log ends %q", code, attempts,
					testCase.wantCode, testCase.wantAttempts, lastLogLine(t, outDir))
			}
			if len(pauses) != attempts-1 || time.Duration(len(pauses))*movePause > time.Second {
				t.Errorf("paused %v between %d moves, want %v before each try after the first",
					pauses, attempts, movePause)
			}
			if code == exitFatal {
				checkKeptPartial(t, outDir)
			}
		})
	}
}

// checkKeptPartial fails the test unless outDir holds the run's files with
// the archive under its partial name, its sidecar naming the final name with
// the partial archive's hash, and a log ending in the FATAL line that says so.
func checkKeptPartial(t *testing.T, outDir string) {
	t.Helper()
	name := runName()
	want := []string{name + ".collection.json", name + ".log", name + ".manifest.jsonl",
		name + ".tar.gz.partial", name + ".tar.gz.sha256"}
	if got := outputNames(t, outDir); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("outputs %v, want %v", got, want)
	}
	partial := filepath.Join(outDir, name+".tar.gz.partial")
	data, err := os.ReadFile(partial)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	sidecar := runOutput(t, outDir, ".sha256")
	if wantLine := hex.EncodeToString(digest[:]) + "  " + name + ".tar.gz\n"; sidecar != wantLine {
		t.Errorf("the sidecar holds %q, want %q", sidecar, wantLine)
	}
	wantLast := "FATAL: archive not moved to its name: rename " + partial + " " +
		filepath.Join(outDir, name+".tar.gz") + ": in use; it is complete as " + partial +
		", with its .sha256 file beside it"
	if last := lastLogLine(t, outDir); !strings.HasSuffix(last, " "+wantLast) {
		t.Errorf("the log ends %q, want %q", last, wantLast)
	}
}
