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

func TestPrintableEscapesEveryControlCharacterAndNothingElse(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"a newline is escaped", "a\nb", `a\x0ab`},
		{"a carriage return is escaped", "a\rb", `a\x0db`},
		{"a terminal escape is escaped", "a\x1b[2Kb", `a\x1b[2Kb`},
		{"a NUL, a tab and DEL are escaped", "\x00\t\x7f", `\x00\x09\x7f`},
		{"a C1 control is escaped as its code point", "a\u0085b\u009bc", `a\u0085b\u009bc`},
		{"the line and paragraph separators are escaped", "a\u2028b\u2029c", `a\u2028b\u2029c`},
		{"each byte that is not valid UTF-8 is escaped", "a\xc2\xffb\xe2\x80", `a\xc2\xffb\xe2\x80`},
		{"other text, a backslash and a letter beyond ASCII included, is kept", `C:\Users\é`,
			`C:\Users\é`},
		{"each bidirectional embedding and override is escaped", "a\u202ab\u202bc\u202cd\u202de\u202ef",
			`a\u202ab\u202bc\u202cd\u202de\u202ef`},
		{"each bidirectional isolate is escaped", "a\u2066b\u2067c\u2068d\u2069e",
			`a\u2066b\u2067c\u2068d\u2069e`},
		{"each bidirectional mark is escaped", "a\u200eb\u200fc\u061cd", `a\u200eb\u200fc\u061cd`},
		{"the neighbours of the escaped characters are kept", "\u00a0\u2027\u202f\u2065\u206a\u200d\u2010\u061b\u061d",
			"\u00a0\u2027\u202f\u2065\u206a\u200d\u2010\u061b\u061d"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := Printable(testCase.text); got != testCase.want {
				t.Errorf("Printable(%q) = %q, want %q", testCase.text, got, testCase.want)
			}
		})
	}
}

func TestEveryLineIsWrittenPrintable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	var stderr bytes.Buffer
	pinned := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	log, err := Open(path, false, &stderr, func() time.Time { return pinned })
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Line("copy failed: a\nforged"); err != nil {
		t.Fatal(err)
	}
	if err := log.FileLine("done: b\r"); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	want := "2026-03-04T05:06:07Z copy failed: a\\x0aforged\n2026-03-04T05:06:07Z done: b\\x0d\n"
	if err != nil || string(got) != want || stderr.String() != "copy failed: a\\x0aforged\n" {
		t.Errorf("file %q and stderr %q (%v), want %q and the first line", got, stderr.String(), err, want)
	}
}

func TestOpenCreatesTheLogNewAndNeverThroughALink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside")
	cases := []struct {
		name  string
		plant func(path string) error
	}{
		{"a file at the name is refused",
			func(path string) error { return os.WriteFile(path, []byte("kept"), 0o600) }},
		{"a link at the name is not followed", func(path string) error { return os.Symlink(outside, path) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(dir, testCase.name)
			if err := testCase.plant(path); err != nil {
				t.Skipf("cannot plant the entry: %v", err)
			}
			if _, err := Open(path, true, &bytes.Buffer{}, time.Now); !errors.Is(err, os.ErrExist) {
				t.Errorf("Open = %v, want the name taken", err)
			}
			if _, err := os.Lstat(outside); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a file was created through the link: %v", err)
			}
		})
	}
}

func TestDiscardWritesOnlyStandardError(t *testing.T) {
	var stderr bytes.Buffer
	log := Discard(false, &stderr, time.Now)
	if err := log.Line("a\nb"); err != nil || log.FileLine("c") != nil || log.Close() != nil {
		t.Fatalf("a discarding log failed: %v", err)
	}
	if stderr.String() != "a\\x0ab\n" {
		t.Errorf("stderr %q, want the escaped line", stderr.String())
	}
}
