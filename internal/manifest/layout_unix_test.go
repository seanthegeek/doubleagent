//go:build unix

package manifest

import (
	"bytes"
	"testing"
)

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
