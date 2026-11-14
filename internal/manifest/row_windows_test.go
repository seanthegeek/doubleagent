package manifest

import (
	"bytes"
	"strings"
	"testing"
)

func TestWindowsRowCarriesFieldsAfterMode(t *testing.T) {
	line := string(appendRow(nil, Row{Mode: "", Owner: "O", Attributes: "Hidden"}))
	want := `"mode":"","owner":"O","attributes":"Hidden","sha256"`
	if !strings.Contains(line, want) {
		t.Errorf("row %s lacks %s", line, want)
	}
}

func TestWriterWritesNativePaths(t *testing.T) {
	var out strings.Builder
	writer := NewWriter(&out)
	if err := writer.Write(Row{Home: "C:/Users/a", Path: "C:/Users/a/x"}); err != nil {
		t.Fatal(err)
	}
	if want := `"home":"C:\\Users\\a","agent":"","path":"C:\\Users\\a\\x"`; !strings.Contains(out.String(), want) {
		t.Errorf("row %s lacks %s", out.String(), want)
	}
	if !writer.HasPath("c:/users/A/X") {
		t.Error("HasPath did not fold")
	}
}

func TestWriterLayout(t *testing.T) {
	row := Row{
		User: "alice", Home: "C:/Users/alice", Agent: "claude-code",
		Path:        "C:/Users/alice/.claude/a\"b\\c\td",
		ArchivePath: "fs/C/Users/alice/.claude/a",
		Type:        "file", Size: 97, Mtime: 3, Atime: 4, Ctime: 5, Btime: 6,
		Mode: "", Owner: `HOST\alice`, Attributes: "Archive",
		SHA256: "abc", Secret: true, Status: Collected,
	}
	want := `{"user":"alice","home":"C:\\Users\\alice","agent":"claude-code",` +
		`"path":"C:\\Users\\alice\\.claude\\a\"b\\c\td","archive_path":"fs/C/Users/alice/.claude/a",` +
		`"type":"file","size":97,"mtime":3,"atime":4,"ctime":5,"btime":6,"uid":0,` +
		`"gid":0,"mode":"","owner":"HOST\\alice","attributes":"Archive","sha256":"abc",` +
		`"secret":true,"status":"collected","target":"","error":""}` + "\n"
	var out bytes.Buffer
	if err := NewWriter(&out).Write(row); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Errorf("row =\n%s want\n%s", out.String(), want)
	}
}
