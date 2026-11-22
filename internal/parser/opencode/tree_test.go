package opencode

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A message id is unique, so message files that claim one id are damage:
// the id's part directory is read for the first of them, in the order the
// files are parsed, and each later one gives its own rows and one problem
// row, so the rows grow with the files rather than with the square of them.
func TestMessageFilesClaimingOneIDShowItsPartsOnce(t *testing.T) {
	const storage = ".local/share/opencode/storage"
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d message files and %d parts of one id", count, count), func(t *testing.T) {
			files := map[string]string{}
			var rels []string
			var want []string
			for i := range count {
				rel := fmt.Sprintf("%s/message/ses_x/msg_%03d.json", storage, i)
				rels = append(rels, rel)
				files[rel] = `{"id": "msg_x", "sessionID": "ses_x", "role": "assistant"}`
				files[fmt.Sprintf("%s/part/msg_x/prt_%03d.json", storage, i)] =
					fmt.Sprintf(`{"type": "text", "text": "part %d"}`, i)
				want = append(want, fmt.Sprintf("part %d", i))
			}
			for range count - 1 {
				want = append(want, "parser: message msg_x repeats the id of an earlier message file; "+
					"its parts are shown with /home/u/"+rels[0])
			}
			home := writeTree(t, files)
			collectedHome := &input.Home{DiskPath: home}
			subject := New()
			var got []string
			for _, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Original: "/home/u/" + rel, Home: collectedHome}
				if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
					got = append(got, row.Text)
				}); err != nil {
					t.Fatal(err)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("%d rows ending %q, want %d rows ending %q", len(got), got[len(got)-1], len(want),
					want[len(want)-1])
			}
			again := 0
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rels[0])), Rel: rels[0],
				Original: "/home/u/" + rels[0], Home: collectedHome}
			if err := subject.Parse(artifact, parser.Options{}, func(timeline.Row) { again++ }); err != nil {
				t.Fatal(err)
			}
			if again != count {
				t.Errorf("the first message file parsed again gave %d rows, want its %d parts", again, count)
			}
		})
	}
}
