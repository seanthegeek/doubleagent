//go:build !windows

package input

import (
	"path/filepath"
	"testing"
)

func TestAPOSIXRowWithABackslashIsFoundInACollectionDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifest(t, root, collectedRow("alice", "/home/alice", "claude-code", `/home/alice/.claude/a\b.jsonl`,
		`fs/home/alice/.claude/a\b.jsonl`))
	writeFile(t, filepath.Join(root, "fs", "home", "alice", ".claude", `a\b.jsonl`), "{}")
	collection, err := Open(t.Context(), root, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantSlice(t, "problems", collection.Problems, nil)
	wantSlice(t, "artifacts", rels(collection), []string{`.claude/a\b.jsonl`})
}
