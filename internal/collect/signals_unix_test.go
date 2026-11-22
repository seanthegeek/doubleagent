//go:build unix

package collect_test

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// signalledFiles is how many files the signal tests' home holds. Each gives
// a match line of more than 200 bytes on standard error, so what the run
// prints after its first user line is several times the largest pipe buffer
// Linux or macOS gives a new pipe (64 KiB).
const signalledFiles = 1024

func TestASignalMidRunStopsTheRunOnlyWhenItAsksTheRunToStop(t *testing.T) {
	binary := buildBinary(t)
	root := signalledImage(t)
	cases := []struct {
		name     string
		signal   syscall.Signal
		wantCode int
	}{
		{"a terminal hangup lets the run finish its archive", syscall.SIGHUP, 0},
		{"a broken pipe signal sent by another program, which Go drops, lets the run finish its archive",
			syscall.SIGPIPE, 0},
		{"a user signal Go drops lets the run finish its archive", syscall.SIGUSR1, 0},
		{"an interrupt stops the run and removes its staging", syscall.SIGINT, 130},
		{"a termination request stops the run and removes its staging", syscall.SIGTERM, 130},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			outDir := t.TempDir()
			code := runSignalled(t, binary, root, outDir, testCase.signal)
			if code != testCase.wantCode {
				t.Fatalf("the run exited %d, want %d", code, testCase.wantCode)
			}
			if testCase.wantCode == 0 {
				checkFinishedOutput(t, outDir)
				return
			}
			checkInterruptedOutput(t, outDir)
		})
	}
}

// signalledImage builds an image whose one home holds signalledFiles Claude
// Code settings copies, each its own catalog match, and returns its root.
func signalledImage(test *testing.T) string {
	test.Helper()
	root := test.TempDir()
	home := filepath.Join(root, "home", "alice")
	if err := os.MkdirAll(home, 0o755); err != nil {
		test.Fatal(err)
	}
	suffix := strings.Repeat("x", 200)
	for i := range signalledFiles {
		name := filepath.Join(home, ".claude.json."+suffix+strconv.Itoa(i))
		if err := os.WriteFile(name, []byte("{}\n"), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	return root
}

// runSignalled runs collect over root into outDir and sends it signal once
// it has printed its first user line, then reads the rest of its standard
// error and returns its exit code. Until the test reads on, the run blocks
// on a full pipe long before its walk ends, so the signal arrives mid-walk
// however fast or slow the machine is.
func runSignalled(test *testing.T, binary, root, outDir string, signal syscall.Signal) int {
	test.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		test.Fatal(err)
	}
	defer reader.Close()
	command := exec.Command(binary, "collect", "-r", root, "-o", outDir)
	command.Stderr = writer
	if err := command.Start(); err != nil {
		test.Fatal(err)
	}
	writer.Close()
	lines := bufio.NewReader(reader)
	for {
		line, err := lines.ReadString('\n')
		if err != nil {
			command.Wait()
			test.Fatalf("the run ended before its first user line: %v", err)
		}
		if strings.HasPrefix(line, "User alice ") {
			break
		}
	}
	if err := command.Process.Signal(signal); err != nil {
		test.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, lines); err != nil {
		test.Fatal(err)
	}
	return exitCode(test, command.Wait())
}

// checkInterruptedOutput fails the test unless outDir holds a log ending with
// the interrupted line and the manifest, and nothing else: no archive, no
// partial archive or sidecar, no collection.json and no staging directory.
func checkInterruptedOutput(test *testing.T, outDir string) {
	test.Helper()
	entries, err := os.ReadDir(outDir)
	if err != nil {
		test.Fatal(err)
	}
	var logs []string
	for _, entry := range entries {
		switch name := entry.Name(); {
		case strings.HasSuffix(name, "_agent-artifacts.log"):
			logs = append(logs, name)
		case !strings.HasSuffix(name, "_agent-artifacts.manifest.jsonl"):
			test.Errorf("the interrupted run left %s", name)
		}
	}
	if len(logs) != 1 {
		test.Fatalf("the run left the logs %v, want one", logs)
	}
	text, err := os.ReadFile(filepath.Join(outDir, logs[0]))
	if err != nil {
		test.Fatal(err)
	}
	if !strings.HasSuffix(string(text), " interrupted\n") {
		test.Errorf("the log does not end with the interrupted line:\n%s", text)
	}
}
