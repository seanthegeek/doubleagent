//go:build unix

package hostcmd

import (
	"context"
	"errors"
	"os"
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
