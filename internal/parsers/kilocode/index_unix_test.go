//go:build !windows

package kilocode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

func TestAnIndexThatCannotBeReadIsReportedForEachTask(t *testing.T) {
	home := t.TempDir()
	index := filepath.Join(home, ".config/Code/User/globalStorage/kilocode.kilo-code/tasks/_index.json")
	write(t, index, `{"entries": []}`)
	if err := os.Chmod(index, 0); err != nil {
		t.Fatal(err)
	}
	var probe parserkit.Problems
	if _, ok := parserkit.FileBytes(index, &probe); ok {
		t.Skip("this user reads a file without read permission")
	}
	subject := New()
	for _, taskID := range []string{"a", "b"} {
		artifact := taskArtifact(home, taskID)
		write(t, artifact.DiskPath, `[]`)
		var problems parserkit.Problems
		subject.historyItems(artifact, taskID, &problems)
		if len(problems) != 1 || problems[0].Message != probe[0].Message {
			t.Errorf("task %s: problems %+v, want the index's %+v", taskID, problems, probe)
		}
	}
}
