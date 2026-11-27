//go:build unix

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/analyze"
)

// collectedArchive runs binary's collect over a one-home image under work
// and returns the path of the archive it wrote.
func collectedArchive(t *testing.T, binary, work string) string {
	t.Helper()
	home := filepath.Join(work, "image", "home", "alice")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(work, "out")
	collect := exec.Command(binary, "collect", "-q", "-r", filepath.Join(work, "image"), "-o", outDir)
	if output, err := collect.CombinedOutput(); err != nil {
		t.Fatalf("collect: %v\n%s", err, output)
	}
	archives, _ := filepath.Glob(filepath.Join(outDir, "*.tar.gz"))
	if len(archives) != 1 || !strings.HasSuffix(archives[0], "_agent-artifacts.tar.gz") {
		t.Fatalf("collect wrote the archives %v, want one", archives)
	}
	return archives[0]
}

func TestAnAnalysisWhoseOutputReaderEndedExitsByItselfAndRemovesItsExtraction(t *testing.T) {
	work := t.TempDir()
	binary := filepath.Join(work, "doubleagent")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the binary: %v\n%s", err, output)
	}
	archive := collectedArchive(t, binary, work)
	cases := []struct {
		name string
		args []string
	}{
		{"detect", []string{"detect", archive}},
		{"timeline", []string{"timeline", archive, filepath.Join(work, "timeline")}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name+" exits with its own code and leaves no extracted directory", func(t *testing.T) {
			temporary := t.TempDir()
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			reader.Close()
			defer writer.Close()
			command := exec.Command(binary, testCase.args...)
			command.Stdout = writer
			command.Env = append(os.Environ(), "TMPDIR="+temporary)
			var stderr strings.Builder
			command.Stderr = &stderr
			var exitErr *exec.ExitError
			err = command.Run()
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != analyze.ExitError ||
				!strings.HasPrefix(stderr.String(), "error: the output could not be written: ") {
				t.Errorf("the command ended with %v and stderr %q, want exit %d and the output error", err,
					stderr.String(), analyze.ExitError)
			}
			if left, _ := filepath.Glob(filepath.Join(temporary, "doubleagent-*")); len(left) != 0 {
				t.Errorf("the command left its extraction %v", left)
			}
		})
	}
}
