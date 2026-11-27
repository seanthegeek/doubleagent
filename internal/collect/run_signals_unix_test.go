//go:build unix

package collect_test

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/collect"
)

// signalledImage builds an image whose one home holds a Claude Code
// settings file and returns its root.
func signalledImage(test *testing.T) string {
	test.Helper()
	root := test.TempDir()
	home := filepath.Join(root, "home", "alice")
	if err := os.MkdirAll(home, 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}\n"), 0o644); err != nil {
		test.Fatal(err)
	}
	return root
}

// runSignalled runs collect over root into outDir as the test binary's
// TestSignalHelperProcess, started through sh with the signals ignored names
// ignored (trap), sends it signal while the run pauses after finding its
// homes, and returns its exit code. The pause ends when the run is told to
// stop; when stops is false the test also ends the pause, after the signal.
func runSignalled(test *testing.T, root, outDir, ignored string, signal syscall.Signal, stops bool) int {
	test.Helper()
	releaseReader, releaseWriter, err := os.Pipe()
	if err != nil {
		test.Fatal(err)
	}
	pausedReader, pausedWriter, err := os.Pipe()
	if err != nil {
		test.Fatal(err)
	}
	defer pausedReader.Close()
	defer releaseWriter.Close()
	// exec keeps an ignored signal ignored in the program it starts.
	script := `exec "$0" "$@"`
	if ignored != "" {
		script = `trap '' ` + ignored + "; " + script
	}
	command := exec.Command("/bin/sh", "-c", script, os.Args[0], "-test.run=^TestSignalHelperProcess$")
	command.Env = append(os.Environ(), collect.SignalHelperRoot+"="+root, collect.SignalHelperOutDir+"="+outDir)
	command.ExtraFiles = []*os.File{releaseReader, pausedWriter}
	if err := command.Start(); err != nil {
		test.Fatal(err)
	}
	releaseReader.Close()
	pausedWriter.Close()
	limit := time.AfterFunc(signalledLimit, func() { _ = command.Process.Kill() })
	defer limit.Stop()
	if _, err := bufio.NewReader(pausedReader).ReadString('\n'); err != nil {
		command.Wait()
		test.Fatalf("the run ended before it found its homes: %v", err)
	}
	if err := command.Process.Signal(signal); err != nil {
		test.Fatal(err)
	}
	if !stops {
		releaseWriter.Close()
	}
	return exitCode(test, command.Wait())
}

// signalledLimit bounds a signalled run, so a run the signal did not stop
// while it waits for one fails the test instead of hanging it.
const signalledLimit = 2 * time.Minute

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

func TestASignalMidRunStopsTheRunOnlyWhenItAsksTheRunToStop(t *testing.T) {
	root := signalledImage(t)
	cases := []struct {
		name string
		// ignored names the signals the run is started with ignored, as
		// nohup or a background job of a shell starts it.
		ignored  string
		signal   syscall.Signal
		wantCode int
	}{
		{"a terminal hangup lets the run finish its archive", "", syscall.SIGHUP, 0},
		{"a broken pipe signal sent by another program, which Go drops, lets the run finish its archive",
			"", syscall.SIGPIPE, 0},
		{"a user signal Go drops lets the run finish its archive", "", syscall.SIGUSR1, 0},
		{"an interrupt stops the run and removes its staging", "", syscall.SIGINT, 130},
		{"a termination request stops the run and removes its staging", "", syscall.SIGTERM, 130},
		{"an interrupt ignored when the run started stays ignored and the run finishes its archive",
			"INT", syscall.SIGINT, 0},
		{"a hangup ignored when the run started lets the run finish its archive", "HUP", syscall.SIGHUP, 0},
		{"a termination request still stops a run started with interrupts and hangups ignored",
			"INT HUP", syscall.SIGTERM, 130},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			outDir := t.TempDir()
			code := runSignalled(t, root, outDir, testCase.ignored, testCase.signal, testCase.wantCode != 0)
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
