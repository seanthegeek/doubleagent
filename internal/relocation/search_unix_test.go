//go:build unix

package relocation

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSearchSkipsAPipeWithoutBlocking(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "environment")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	run := &search{names: []string{"CODEX_HOME"}}
	cases := []struct {
		name string
		read func() int
	}{
		{"the name search reads nothing from a pipe", func() int {
			return len(run.boundedNamesOf(pipe))
		}},
		{"the value reader reads nothing from a pipe", func() int {
			hits := []LineName{{Line: 1, Name: "CODEX_HOME"}}
			values, _ := run.resolve(context.Background(), pipe, "environment", "", hits)
			return len(values.Assignments)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			done := make(chan int, 1)
			go func() { done <- test.read() }()
			select {
			case got := <-done:
				if got != 0 {
					t.Errorf("got %d results from a pipe, want 0", got)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("reading a pipe at a startup file's path did not return")
			}
		})
	}
}

// linkOut makes link a symlink to the directory target, both native.
func linkOut(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
