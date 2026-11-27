package cursor

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// rowsAt writes text at rel under a new home and returns whether the
// parser wants the file and, when it does, the rows it gives, each without
// its source file.
func rowsAt(t *testing.T, rel, text string) (bool, []timeline.Row) {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "cursor",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	parser := New()
	if !parser.Wants(artifact) {
		return false, nil
	}
	if err := parser.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
		row.SourceFile = ""
		rows = append(rows, row)
	}); err != nil {
		t.Fatal(err)
	}
	return true, rows
}

// The fixed parts of a path match in any ASCII case, as the catalog matches
// them, and the session id taken from the file name keeps the disk's
// spelling.
func TestAPathSpelledInAnotherCaseIsRead(t *testing.T) {
	const transcript = `{"role": "user", "message": {"content": "hi"}}` + "\n"
	_, want := rowsAt(t, ".cursor/projects/p/agent-transcripts/c/Ab1.jsonl", transcript)
	wanted, got := rowsAt(t, ".CURSOR/Projects/p/Agent-Transcripts/c/Ab1.JSONL", transcript)
	if !wanted || len(want) == 0 || !reflect.DeepEqual(got, want) || got[0].SessionID != "Ab1" {
		t.Errorf("wanted %v, got %+v, want %+v", wanted, got, want)
	}
}

func TestARowNamesAnAttemptFileAsThePathSpellsIt(t *testing.T) {
	wanted, got := rowsAt(t, ".config/Cursor/User/globalStorage/MCP-OAuth-Attempts/A0.JSON", `{"createdAtMs": 1790845200000}`)
	if !wanted || len(got) != 1 || !strings.HasSuffix(got[0].Text, "(from MCP-OAuth-Attempts/A0.JSON)") {
		t.Errorf("wanted %v, got %+v", wanted, got)
	}
}
