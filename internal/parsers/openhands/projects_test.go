package openhands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestEveryProjectHistoryLooksItsDigestUpInOneReadOfTheConversations(t *testing.T) {
	const projects = 20
	files := map[string]string{}
	var histories []string
	for i := range projects {
		project := fmt.Sprintf("/srv/project%02d", i)
		sum := sha256.Sum256([]byte(project))
		history := ".openhands/projects/" + hex.EncodeToString(sum[:]) + "/prompt_history.json"
		histories = append(histories, history)
		files[history] = `[{"text": "hello"}]`
		files[fmt.Sprintf(".openhands/conversations/c%02d/base_state.json", i)] =
			`{"workspace": {"working_dir": "` + project + `"}}`
	}
	home := writeTree(t, t.TempDir(), files)
	subject := New()
	subject.Work = parserkit.CountingWork()
	for i, history := range histories {
		want := fmt.Sprintf("/srv/project%02d", i)
		artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(history)), Rel: history,
			Home: &input.Home{DiskPath: home}}
		var rows []timeline.Row
		if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].ProjectPath != want {
			t.Fatalf("%s: rows %+v, want one in %s", history, rows, want)
		}
	}
	if reads := subject.Work.Count("conversations read for projects"); reads != projects {
		t.Errorf("%d histories read %d conversations, want each of the %d once", projects, reads, projects)
	}
}
