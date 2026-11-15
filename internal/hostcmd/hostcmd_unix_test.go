//go:build unix

package hostcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// This runs /bin/sh, which Windows does not have.

func TestSystemRunsProgramsInTheCLocale(t *testing.T) {
	cases := []struct {
		name string
		run  func(Runner) (string, error)
		want string
	}{
		{"sh sees LC_ALL=C", func(runner Runner) (string, error) {
			output, err := runner.Output(context.Background(), "/bin/sh", "-c", "printf %s \"$LC_ALL\"")
			return string(output), err
		}, "C"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.run(System())
			if err != nil || got != testCase.want {
				t.Fatalf("got %q, %v; want %q", got, err, testCase.want)
			}
		})
	}
}

func TestSystemStopsAProgramWhenTheRunIsCancelled(t *testing.T) {
	cases := []struct {
		name    string
		program string
		args    []string
	}{
		{"a program is killed", "sleep", []string{"30"}},
		{"a shell whose child holds the output pipe is let go", "/bin/sh",
			[]string{"-c", "sleep 5; :"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			started := time.Now()
			_, err := System().Output(ctx, testCase.program, testCase.args...)
			elapsed := time.Since(started)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("error = %v, want one wrapping the context's", err)
			}
			if elapsed > 3*time.Second {
				t.Errorf("Output returned after %v, want about a second at most", elapsed)
			}
		})
	}
}

func TestSystemStartsNothingOnceCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	marker := t.TempDir() + "/ran"
	_, err := System().Output(ctx, "/bin/sh", "-c", "echo > "+marker)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Lstat(marker); statErr == nil {
		t.Error("the program ran after the context was cancelled")
	}
}

// writeProgram writes an executable sh script named name in directory that
// prints text.
func writeProgram(t *testing.T, directory, name, text string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	script := "#!/bin/sh\nprintf '%s\\n' '" + text + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLookPathPrefersTheSystemDirectories(t *testing.T) {
	planted := t.TempDir()
	writeProgram(t, planted, "echo", "planted")
	onlyOnPath := writeProgram(t, planted, "doubleagent-path-only", "found")
	searchPath := planted + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Run("a planted program earlier in PATH loses to the system one", func(t *testing.T) {
		program, err := lookPath("echo", searchPath)
		if err != nil || filepath.Dir(program) == planted ||
			!slices.Contains(systemDirectories(), filepath.Dir(program)) {
			t.Errorf("lookPath = %q, %v; want echo from a system directory", program, err)
		}
	})
	t.Run("a program only on PATH is still found", func(t *testing.T) {
		if program, err := lookPath("doubleagent-path-only", searchPath); program != onlyOnPath {
			t.Errorf("lookPath = %q, %v; want %q", program, err, onlyOnPath)
		}
	})
	t.Run("a relative PATH entry is not searched", func(t *testing.T) {
		t.Chdir(planted)
		if program, err := lookPath("doubleagent-path-only", "."); err == nil {
			t.Errorf("lookPath = %q, want not found", program)
		}
	})
}

func TestSystemRunsTheProgramLookPathFinds(t *testing.T) {
	planted := t.TempDir()
	writeProgram(t, planted, "echo", "planted")
	writeProgram(t, planted, "doubleagent-path-only", "found")
	t.Setenv("PATH", planted+string(os.PathListSeparator)+os.Getenv("PATH"))
	cases := []struct {
		name, program, want string
	}{
		{"the system echo runs, not the planted one", "echo", "real\n"},
		{"a program only on PATH runs", "doubleagent-path-only", "found\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if !System().Available(testCase.program) {
				t.Errorf("%s is not available", testCase.program)
			}
			output, err := System().Output(context.Background(), testCase.program, "real")
			if err != nil || string(output) != testCase.want {
				t.Errorf("Output = %q, %v; want %q", output, err, testCase.want)
			}
		})
	}
}
