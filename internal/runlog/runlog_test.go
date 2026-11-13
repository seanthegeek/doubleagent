package runlog

import (
	"bytes"
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
