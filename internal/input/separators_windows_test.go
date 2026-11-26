package input

import (
	"fmt"
	"testing"
)

func TestABackslashSeparatesInEveryZipMemberOnWindows(t *testing.T) {
	t.Parallel()
	names := []string{`alice/.claude/a\b.jsonl`, `alice/.claude/d\`}
	for _, system := range []uint16{0, 3, 10, 19} {
		t.Run(fmt.Sprintf("system %d", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipMadeBy(t, system, names,
				map[string]string{names[0]: "{}"}))
			opened(t, err)
			wantSlice(t, "problems", collection.Problems, nil)
			wantSlice(t, "artifacts", rels(collection), []string{".claude/a/b.jsonl"})
		})
	}
}
