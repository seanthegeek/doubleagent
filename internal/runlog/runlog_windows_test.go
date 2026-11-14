package runlog

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogErrorsNameTheNativePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	log, err := Open(path, true, io.Discard, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	// Writing to and closing the closed file fails with the file's name,
	// which osfile opened in its \\?\ form.
	cases := []struct {
		name string
		call func() error
	}{
		{"a line to the closed file", func() error { return log.Line("x") }},
		{"a file-only line to the closed file", func() error { return log.FileLine("x") }},
		{"closing the closed file", log.Close},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if err == nil || strings.Contains(err.Error(), `\\?\`) || !strings.Contains(err.Error(), path) {
				t.Errorf("error = %v, want one naming %s without the \\\\?\\ prefix", err, path)
			}
		})
	}
}
