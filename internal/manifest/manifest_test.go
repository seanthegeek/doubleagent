package manifest

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestEscapeJSON(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"plain text", "/home/alice/.claude", "/home/alice/.claude"},
		{"quote and backslash", `a"b\c`, `a\"b\\c`},
		{"named controls", "\b\t\n\f\r", `\b\t\n\f\r`},
		{"other controls as lower-case hex", "\x00\x01\x0b\x1f", `\u0000\u0001\u000b\u001f`},
		{"delete passes", "a\x7fb", "a\x7fb"},
		{"valid UTF-8 passes", "café  ", "café  "},
		{"invalid UTF-8 passes", "a\xff\xfeb", "a\xff\xfeb"},
		{"slash is not escaped", "a/b", "a/b"},
		{"empty", "", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := EscapeJSON(testCase.text); got != testCase.want {
				t.Errorf("EscapeJSON(%q) = %q, want %q", testCase.text, got, testCase.want)
			}
		})
	}
}

func TestWriterLayout(t *testing.T) {
	row := Row{
		User: "alice", Home: "/home/alice", Agent: "claude-code",
		Path:        "/home/alice/.claude/a\"b\\c\td",
		ArchivePath: "fs/home/alice/.claude/a",
		Type:        "file", Size: 97, Mtime: 3, Atime: 4, Ctime: 5, Btime: 6,
		UID: 1000, GID: 1001, Mode: "644", SHA256: "abc", Secret: true,
		Status: Collected, Target: "", Error: "",
	}
	want := `{"user":"alice","home":"/home/alice","agent":"claude-code",` +
		`"path":"/home/alice/.claude/a\"b\\c\td","archive_path":"fs/home/alice/.claude/a",` +
		`"type":"file","size":97,"mtime":3,"atime":4,"ctime":5,"btime":6,"uid":1000,` +
		`"gid":1001,"mode":"644","sha256":"abc","secret":true,"status":"collected",` +
		`"target":"","error":""}` + "\n"
	var out bytes.Buffer
	if err := NewWriter(&out).Write(row); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Errorf("row =\n%s want\n%s", out.String(), want)
	}
}

func TestWriterNegativeNumbersAreZero(t *testing.T) {
	var out bytes.Buffer
	if err := NewWriter(&out).Write(Row{Mtime: -5, Size: -1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"size":0,"mtime":0,`) {
		t.Errorf("row = %s, want negative numbers written as 0", out.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriterBookkeeping(t *testing.T) {
	var out bytes.Buffer
	writer := NewWriter(&out)
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
	homes := writer.HomesWithRows()
	if _, ok := homes["/h/a"]; ok || len(homes) != 2 {
		t.Errorf("homes = %v, want /h/b and /h/c only", homes)
	}
	if _, ok := homes["/h/b"]; !ok {
		t.Errorf("homes = %v, want /h/b", homes)
	}
	if got := strings.Count(out.String(), "\n"); got != len(rows) {
		t.Errorf("lines = %d, want %d", got, len(rows))
	}
}

func TestWriterFailedWriteIsNotCounted(t *testing.T) {
	writer := NewWriter(failingWriter{})
	if err := writer.Write(Row{Path: "/x", Status: Collected}); err == nil {
		t.Fatal("want the write error")
	}
	if writer.HasPath("/x") || writer.Counts().ByStatus[Collected] != 0 {
		t.Error("a row that was not written was counted")
	}
}
