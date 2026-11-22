package runlog

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLineWritesTheTimedLineToTheFileAndTheBareLineToStderr(t *testing.T) {
	cases := []struct {
		name       string
		quiet      bool
		wantStderr string
	}{
		{"a loud log echoes to stderr", false, "hello world\n"},
		{"a quiet log writes only the file", true, ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "run.log")
			var stderr bytes.Buffer
			pinned := time.Date(2026, 3, 4, 5, 6, 7, 999, time.FixedZone("x", 3600))
			log, err := Open(path, testCase.quiet, &stderr, func() time.Time { return pinned })
			if err != nil {
				t.Fatal(err)
			}
			if err := log.Line("hello world"); err != nil {
				t.Fatal(err)
			}
			if err := log.Close(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if want := "2026-03-04T04:06:07Z hello world\n"; string(got) != want {
				t.Errorf("file %q, want %q", got, want)
			}
			if stderr.String() != testCase.wantStderr {
				t.Errorf("stderr %q, want %q", stderr.String(), testCase.wantStderr)
			}
		})
	}
}

func TestOpenReportsAnUnwritablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "run.log")
	if _, err := Open(path, true, &bytes.Buffer{}, time.Now); err == nil {
		t.Fatal("want an error for a path in a missing directory")
	}
}

func TestFileLineWritesOnlyTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	var stderr bytes.Buffer
	pinned := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	log, err := Open(path, false, &stderr, func() time.Time { return pinned })
	if err != nil {
		t.Fatal(err)
	}
	if err := log.FileLine("done: x"); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-03-04T05:06:07Z done: x\n"; string(got) != want || stderr.Len() != 0 {
		t.Errorf("file %q and stderr %q, want %q and nothing", got, stderr.String(), want)
	}
}

// failingWriter is a standard error whose every write fails, as a write to
// a closed pipe does.
type failingWriter struct{}

// Write fails.
func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestLineDropsAFailedWriteToStandardError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	log, err := Open(path, false, failingWriter{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Line("still logged"); err != nil {
		t.Errorf("Line returned %v for a failed standard error, want nil", err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.HasSuffix(got, []byte(" still logged\n")) {
		t.Errorf("file %q (%v), want the line", got, err)
	}
}
