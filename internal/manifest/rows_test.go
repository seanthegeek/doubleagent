package manifest

import "testing"

// TestRowsOfEachStatusShape writes the three shapes of a home's agent files: a
// collected credential file with its hash, a symbolic link whose target holds
// a quote and a backslash, and a file skipped for its size.
func TestRowsOfEachStatusShape(t *testing.T) {
	const home = "/img/home/alice"
	cases := []struct {
		name string
		row  Row
		want string
	}{
		{"a collected credential file", Row{
			User: "alice", Home: home, Agent: "claude-code", Path: home + "/.claude/settings.json",
			ArchivePath: "fs" + home + "/.claude/settings.json", Type: "file", Size: 9,
			Mtime: 1700000000, Atime: 1700000100, Ctime: 1700000200, Btime: 1699999999,
			UID: 1000, GID: 1000, Mode: "644",
			SHA256: "bbd7a96d4be3a2cfd4d1ff1bd0c5f3ba0c0ae7a3c2b7a1e6bf4a3c4d3c8e9f10",
			Secret: true, Status: Collected,
		}, `{"user":"alice","home":"/img/home/alice","agent":"claude-code",` +
			`"path":"/img/home/alice/.claude/settings.json",` +
			`"archive_path":"fs/img/home/alice/.claude/settings.json","type":"file",` +
			`"size":9,"mtime":1700000000,"atime":1700000100,"ctime":1700000200,"btime":1699999999,` +
			`"uid":1000,"gid":1000,"mode":"644",` + platformFieldsText +
			`"sha256":"bbd7a96d4be3a2cfd4d1ff1bd0c5f3ba0c0ae7a3c2b7a1e6bf4a3c4d3c8e9f10",` +
			`"secret":true,"status":"collected","target":"","error":""}` + "\n"},
		{"a symbolic link with a quote and a backslash in its target", Row{
			User: "alice", Home: home, Agent: "claude-code", Path: home + "/.claude/plugins",
			ArchivePath: "fs" + home + "/.claude/plugins", Type: "symlink",
			Mtime: 1700000000, Atime: 1700000000, Ctime: 1700000000, UID: 1000, GID: 1000,
			Mode: "777", Status: Symlink, Target: `target "q" \ end`,
		}, `{"user":"alice","home":"/img/home/alice","agent":"claude-code",` +
			`"path":"/img/home/alice/.claude/plugins",` +
			`"archive_path":"fs/img/home/alice/.claude/plugins","type":"symlink",` +
			`"size":0,"mtime":1700000000,"atime":1700000000,"ctime":1700000000,"btime":0,` +
			`"uid":1000,"gid":1000,"mode":"777",` + platformFieldsText +
			`"sha256":"","secret":false,"status":"symlink","target":"target \"q\" \\ end","error":""}` + "\n"},
		{"a file skipped for its size", Row{
			User: "alice", Home: home, Agent: "claude-code", Path: home + "/.claude/CLAUDE.md",
			Type: "file", Size: 1500000, Mtime: 1700000000, Atime: 1700000000,
			Ctime: 1700000000, UID: 1000, GID: 1000, Mode: "644", Status: SkippedSize,
		}, `{"user":"alice","home":"/img/home/alice","agent":"claude-code",` +
			`"path":"/img/home/alice/.claude/CLAUDE.md","archive_path":"","type":"file",` +
			`"size":1500000,"mtime":1700000000,"atime":1700000000,"ctime":1700000000,"btime":0,` +
			`"uid":1000,"gid":1000,"mode":"644",` + platformFieldsText +
			`"sha256":"","secret":false,"status":"skipped_size","target":"","error":""}` + "\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := lineOf(t, test.row); got != test.want {
				t.Errorf("row:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}
