//go:build unix

package collect_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// exitCode returns the exit code a finished command's error gives, failing
// the test on a command that did not exit by itself (killed by a signal, or
// never started).
func exitCode(test *testing.T, err error) int {
	test.Helper()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exitErr) && exitErr.ExitCode() >= 0:
		return exitErr.ExitCode()
	}
	test.Fatalf("the run did not exit by itself: %v", err)
	return 0
}

// closedReader returns the write end of a pipe whose read end is already
// closed when closed is set, so every write to it fails as a write to a
// console that went away does, or the null device otherwise.
func closedReader(test *testing.T, closed bool) *os.File {
	test.Helper()
	if !closed {
		null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			test.Fatal(err)
		}
		test.Cleanup(func() { null.Close() })
		return null
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		test.Fatal(err)
	}
	reader.Close()
	test.Cleanup(func() { writer.Close() })
	return writer
}

// checkFinishedOutput fails the test unless outDir holds one run's archive
// and a log ending with its done line, and no staging directory.
func checkFinishedOutput(test *testing.T, outDir string) {
	test.Helper()
	entries, err := os.ReadDir(outDir)
	if err != nil {
		test.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".stage-") {
			test.Errorf("the run left its staging directory %s", entry.Name())
		}
	}
	logs, _ := filepath.Glob(filepath.Join(outDir, "*_agent-artifacts.log"))
	archives, _ := filepath.Glob(filepath.Join(outDir, "*_agent-artifacts.tar.gz"))
	if len(logs) != 1 || len(archives) != 1 {
		test.Fatalf("the run left %d logs and %d archives, want one of each", len(logs), len(archives))
	}
	text, err := os.ReadFile(logs[0])
	if err != nil {
		test.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	if last := lines[len(lines)-1]; !strings.Contains(last, " done: ") {
		test.Errorf("the log ends %q, want its done line", last)
	}
}

func TestARunWhoseConsoleGoesAwayFinishesItsArchive(t *testing.T) {
	binary := buildBinary(t)
	img := imagetest.Build(t, t.TempDir())
	cases := []struct {
		name                     string
		closeStdout, closeStderr bool
		inventory                bool
		wantCode                 int
	}{
		{"a closed standard error drops the progress lines", false, true, false, 0},
		{"a closed standard output drops the summary", true, false, false, 0},
		{"both streams closed drop both", true, true, false, 0},
		{"an inventory with nowhere to go exits 2", true, false, true, 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			outDir := t.TempDir()
			args := []string{"collect", "-r", img.Root, "-o", outDir, "--max-file-size", "1"}
			if testCase.inventory {
				args = []string{"collect", "-r", img.Root, "--inventory"}
			}
			command := exec.Command(binary, args...)
			command.Stdout = closedReader(t, testCase.closeStdout)
			command.Stderr = closedReader(t, testCase.closeStderr)
			if code := exitCode(t, command.Run()); code != testCase.wantCode {
				t.Fatalf("the run exited %d, want %d", code, testCase.wantCode)
			}
			if testCase.inventory {
				return
			}
			checkFinishedOutput(t, outDir)
		})
	}
}
