package input

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
)

func TestABackslashSplitsAManifestPathOnlyInAWindowsRow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, home, path, archivePath, member, wantRel string
	}{
		{name: "a backslash in a POSIX file name is part of the name", home: "/home/alice",
			path: `/home/alice/.claude/a\b.jsonl`, archivePath: `fs/home/alice/.claude/a\b.jsonl`,
			member: `fs/home/alice/.claude/a\b.jsonl`, wantRel: `.claude/a\b.jsonl`},
		{name: "a backslash in a Windows row separates components", home: `C:\Users\bob`,
			path: `C:\Users\bob\.claude\x.jsonl`, archivePath: `fs\C\Users\bob\.claude\x.jsonl`,
			member: "fs/C/Users/bob/.claude/x.jsonl", wantRel: ".claude/x.jsonl"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			manifest := collectedRow("alice", test.home, "claude-code", test.path, test.archivePath) + "\n"
			archive := filepath.Join(dir, "h_agent-artifacts.tar")
			writeFile(t, archive, string(tarBytes(t, false,
				tarMember{name: "manifest.jsonl", typeflag: tar.TypeReg, body: manifest},
				tarMember{name: test.member, typeflag: tar.TypeReg, body: "{}"})))
			collection, err := Open(t.Context(), archive, Options{Matcher: matcherOf(t, "")})
			opened(t, err)
			t.Cleanup(func() { _ = collection.Cleanup() })
			wantSlice(t, "problems", collection.Problems, nil)
			if len(collection.Artifacts) != 1 {
				t.Fatalf("%d artifacts, want 1", len(collection.Artifacts))
			}
			wantEqual(t, "rel", collection.Artifacts[0].Rel, test.wantRel)
		})
	}
}

// zipMadeBy returns a zip archive holding each name of names with its body,
// each member's "version made by" naming the system system.
func zipMadeBy(t *testing.T, system uint16, names []string, bodies map[string]string) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range names {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate,
			CreatorVersion: system << 8})
		opened(t, err)
		_, err = entry.Write([]byte(bodies[name]))
		opened(t, err)
	}
	opened(t, writer.Close())
	return archive.Bytes()
}

func TestABackslashSeparatesInAZipMemberMadeOnMSDOSOrWindows(t *testing.T) {
	t.Parallel()
	manifest := collectedRow("alice", "/home/alice", "claude-code", "/home/alice/.claude/x.jsonl",
		"fs/home/alice/.claude/x.jsonl") + "\n"
	names := []string{"manifest.jsonl", `fs\home\alice\.claude\`, `fs\home\alice\.claude\x.jsonl`}
	for _, system := range []uint16{0, 10, 11, 99} {
		t.Run(fmt.Sprintf("system %d", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipMadeBy(t, system, names,
				map[string]string{"manifest.jsonl": manifest, names[2]: "{}"}))
			opened(t, err)
			wantSlice(t, "problems", collection.Problems, nil)
			wantSlice(t, "artifacts", rels(collection), []string{".claude/x.jsonl"})
		})
	}
}
