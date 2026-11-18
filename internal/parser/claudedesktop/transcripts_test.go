package claudedesktop

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// Every Code tab record looks for its CLI transcript in each slug of
// ~/.claude/projects, so each slug directory is listed once for the run
// and the transcript's name looked up in the listings, not looked at once
// per record and slug.
func TestEachProjectSlugIsListedOnceForAllRecords(t *testing.T) {
	const slugs = 6
	for _, records := range []int{4, 40} {
		t.Run(fmt.Sprintf("%d records over %d slugs make %d listings", records, slugs, slugs), func(t *testing.T) {
			home := t.TempDir()
			for slug := range slugs {
				writeEmpty(t, home, fmt.Sprintf(".claude/projects/slug%d/other.jsonl", slug))
			}
			subject := New()
			subject.Work = parser.CountingWork()
			for i := range records {
				cliID := fmt.Sprintf("c%d", i)
				if i%2 == 0 {
					writeEmpty(t, home, fmt.Sprintf(".claude/projects/slug%d/%s.jsonl", i%slugs, cliID))
				}
			}
			for i := range records {
				rel := cowork + "/" + session + ".json"
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: &input.Home{DiskPath: home}}
				record := jsonvalue.NewObject(
					jsonvalue.Member{Key: "sessionId", Value: jsonvalue.NewString(session)},
					jsonvalue.Member{Key: "cliSessionId", Value: jsonvalue.NewString(fmt.Sprintf("c%d", i))})
				if got, want := subject.sessionCollected(artifact, record), i%2 == 0; got != want {
					t.Errorf("record %d collected %v, want %v", i, got, want)
				}
			}
			if listings := subject.Work.Count("slug directory listings"); listings != slugs {
				t.Errorf("%d records made %d slug directory listings, want %d", records, listings, slugs)
			}
		})
	}
}
