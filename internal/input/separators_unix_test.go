//go:build !windows

package input

import (
	"fmt"
	"testing"
)

func TestABackslashIsPartOfANameInAZipMemberMadeOnUnixOrOSX(t *testing.T) {
	t.Parallel()
	manifest := collectedRow("alice", "/home/alice", "claude-code", `/home/alice/.claude/a\b.jsonl`,
		`fs/home/alice/.claude/a\b.jsonl`) + "\n"
	withManifest := []string{"manifest.jsonl", `fs/home/alice/.claude/a\b.jsonl`}
	loose := []string{`alice/.claude/a\b.jsonl`, `alice/.claude/d\`}
	for _, system := range []uint16{3, 19} {
		t.Run(fmt.Sprintf("system %d with a manifest", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipMadeBy(t, system, withManifest,
				map[string]string{"manifest.jsonl": manifest, withManifest[1]: "{}"}))
			opened(t, err)
			wantSlice(t, "problems", collection.Problems, nil)
			wantSlice(t, "artifacts", rels(collection), []string{`.claude/a\b.jsonl`})
		})
		t.Run(fmt.Sprintf("system %d without a manifest", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipMadeBy(t, system, loose,
				map[string]string{loose[0]: "{}"}))
			opened(t, err)
			wantSlice(t, "problems", collection.Problems, nil)
			wantSlice(t, "artifacts", rels(collection), []string{`.claude/a\b.jsonl`, `.claude/d\`})
		})
	}
}
