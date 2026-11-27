package manifest

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// lineOf writes row to a fresh Writer and returns what it wrote.
func lineOf(t *testing.T, row Row) string {
	t.Helper()
	var out bytes.Buffer
	if err := NewWriter(&out, nil).Write(row); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestRowStringsAreEscapedAsJSON(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"plain text", "/home/alice/.claude", `"/home/alice/.claude"`},
		{"quote and backslash", `a"b\c`, `"a\"b\\c"`},
		{"named controls", "\b\t\n\f\r", `"\b\t\n\f\r"`},
		{"other controls as lower-case hex", "\x00\x01\x0b\x1f", `"\u0000\u0001\u000b\u001f"`},
		{"delete passes", "a\x7fb", "\"a\x7fb\""},
		{"valid UTF-8 passes", "caf\u00e9 \u65e5", "\"caf\u00e9 \u65e5\""},
		{"angle brackets and ampersand pass", "a<b>&c", `"a<b>&c"`},
		{"slash is not escaped", "a/b", `"a/b"`},
		{"line and paragraph separators are escaped", "a\u2028b\u2029c", `"a\u2028b\u2029c"`},
		{"each byte of invalid UTF-8 becomes an escaped replacement character", "a\xff\xfeb",
			"\"a\ufffd\ufffdb\""},
		{"empty", "", `""`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			want := `"target":` + testCase.want + `,`
			if got := lineOf(t, Row{Target: testCase.text}); !strings.Contains(got, want) {
				t.Errorf("row %q lacks %s", got, want)
			}
		})
	}
}

func TestRowFieldsAreWrittenInManifestOrderAndNoneIsOmitted(t *testing.T) {
	got := lineOf(t, Row{})
	want := `{"user":"","home":"","agent":"","path":"","archive_path":"","type":"",` +
		`"size":0,"mtime":0,"atime":0,"ctime":0,"btime":0,"uid":0,"gid":0,"mode":"",` +
		platformFieldsText + `"sha256":"","secret":false,"status":"","target":"","error":""}` + "\n"
	if got != want {
		t.Errorf("empty row = %q, want %q", got, want)
	}
}

func TestRowWithEveryFieldSet(t *testing.T) {
	got := lineOf(t, Row{User: "u", Home: "h", Agent: "a", Path: "p", ArchivePath: "fs/h/p",
		Type: "file", Size: 1, Mtime: 2, Atime: 3, Ctime: 4, Btime: 5, UID: 6, GID: 7, Mode: "644",
		SHA256: "ab", Secret: true, Status: Collected, Target: "t", Error: "e"})
	want := `{"user":"u","home":"h","agent":"a","path":"p","archive_path":"fs/h/p","type":"file",` +
		`"size":1,"mtime":2,"atime":3,"ctime":4,"btime":5,"uid":6,"gid":7,"mode":"644",` +
		platformFieldsText + `"sha256":"ab","secret":true,"status":"collected","target":"t","error":"e"}` + "\n"
	if got != want {
		t.Errorf("row = %q, want %q", got, want)
	}
}

func TestWriterNegativeNumbersAreZero(t *testing.T) {
	var out bytes.Buffer
	if err := NewWriter(&out, nil).Write(Row{Mtime: -5, Size: -1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"size":0,"mtime":0,`) {
		t.Errorf("row = %s, want negative numbers written as 0", out.String())
	}
}

type failingWriter struct{}

// Write fails every write, as a full disk does.
func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriterBookkeeping(t *testing.T) {
	var out bytes.Buffer
	writer := NewWriter(&out, nil)
	rows := []Row{
		{Home: "/h/a", Path: "/h/a", Type: "dir", Status: ErrorRead},
		{Home: "/h/b", Path: "/h/b", Type: "dir", Status: ErrorRead},
		{Home: "/h/b", Path: "/h/b/f", Type: "file", Size: 10, Status: Collected},
		{Home: "/h/b", Path: "/h/b/g", Type: "file", Size: 5, Status: Collected},
		{Home: "/h/c", Path: "/h/c", ArchivePath: "fs/h/c", Type: "dir", Status: SkippedExcluded},
		{Home: "", Path: "/etc/x", Type: "file", Status: Symlink},
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	counts := writer.Counts()
	if counts.ByStatus[Collected] != 2 || counts.ByStatus[ErrorRead] != 2 ||
		counts.ByStatus[Symlink] != 1 || counts.ByStatus[SkippedSize] != 0 {
		t.Errorf("counts = %v", counts.ByStatus)
	}
	if counts.CollectedBytes != 15 {
		t.Errorf("collected bytes = %d, want 15", counts.CollectedBytes)
	}
	for path, want := range map[string]bool{"/h/b/f": true, "/h/b/": false, "/h": false, "/etc/x": true} {
		if got := writer.HasPath(path); got != want {
			t.Errorf("HasPath(%q) = %v, want %v", path, got, want)
		}
	}
	for home, want := range map[string]bool{"/h/a": false, "/h/b": true, "/h/c": true, "": false} {
		if got := writer.HasHomeRows(home); got != want {
			t.Errorf("HasHomeRows(%q) = %v, want %v", home, got, want)
		}
	}
	if got := strings.Count(out.String(), "\n"); got != len(rows) {
		t.Errorf("lines = %d, want %d", got, len(rows))
	}
}

func TestWriterFailedWriteIsNotCounted(t *testing.T) {
	writer := NewWriter(failingWriter{}, nil)
	if err := writer.Write(Row{Path: "/x", Status: Collected}); err == nil {
		t.Fatal("want the write error")
	}
	if writer.HasPath("/x") || writer.Counts().ByStatus[Collected] != 0 {
		t.Error("a row that was not written was counted")
	}
}
