package input

import (
	"archive/tar"
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
