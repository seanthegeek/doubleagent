package walk

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// TestTheOpenerRefusesWhatWindowsOpensOutsideTheRoot runs the walk with a
// root check that lets everything through, as a mistake of the check would,
// over a junction out of the image root, so only the opener's confinement
// stands between the analyst's files and the archive.
func TestTheOpenerRefusesWhatWindowsOpensOutsideTheRoot(t *testing.T) {
	root, outside := physicalTemp(t), physicalTemp(t)
	writeFiles(t, root, "Users/alice/notes.txt")
	writeFiles(t, outside, "secret.txt", "dir/inner.txt")
	makeJunction(t, outside, root+"/Users/alice/j")
	collector, recorder := windowsCollector(t, root, nil)
	collector.RootCheck = RootCheck{}
	confinement, err := follow.NewConfinement(root, func(path string) {
		_ = recorder.Line("refused " + path)
	})
	mustDo(t, err)
	collector.Stager.Opener = &fsmeta.Opener{Admit: confinement.Admit}
	home, ctx := root+"/Users/alice", context.Background()
	for _, path := range []string{home + "/notes.txt", home + "/j/secret.txt",
		home + "/j/secret.txt"} {
		_, err := collector.CollectPath(ctx, "alice", home, "agent", path)
		mustDo(t, err)
	}
	_, err = collector.CollectTree(ctx, "alice", home, "agent", home+"/j/dir")
	mustDo(t, err)
	if row := recorder.rowOf(t, home+"/notes.txt"); row["status"] != "collected" {
		t.Errorf("the file inside the root was not collected: %v", row)
	}
	for _, row := range recorder.rows(t) {
		path, _ := row["path"].(string)
		if strings.HasSuffix(path, `\secret.txt`) || strings.HasSuffix(path, `\inner.txt`) {
			t.Errorf("a row for a file outside the root: %v", row)
		}
	}
	var refused []string
	for _, event := range recorder.written {
		if strings.HasPrefix(event, "log refused ") {
			refused = append(refused, strings.TrimPrefix(event, "log refused "))
		}
	}
	want := []string{home + "/j/secret.txt", home + "/j/dir"}
	if !slices.Equal(refused, want) {
		t.Errorf("refused %q, want %q", refused, want)
	}
	staged := false
	mustDo(t, walkFiles(collector.Stager.Dir, func(path string) {
		staged = staged || strings.HasSuffix(path, "secret.txt") || strings.HasSuffix(path, "inner.txt")
	}))
	if staged {
		t.Error("a file outside the root was staged")
	}
}

// walkFiles calls each with every regular file below dir.
func walkFiles(dir string, each func(path string)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := dir + "/" + entry.Name()
		if entry.IsDir() {
			if err := walkFiles(path, each); err != nil {
				return err
			}
		} else if entry.Type().IsRegular() {
			each(path)
		}
	}
	return nil
}
