package vscode

import (
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// The fixed parts of a path match in any ASCII case, as the catalog matches
// them, and the session id taken from the file name keeps the disk's
// spelling.
func TestAPathSpelledInAnotherCaseIsRead(t *testing.T) {
	const session = `{"requests": [{"message": {"text": "hi"}, "timestamp": 1791021600000}]}`
	const respelled = ".CONFIG/code/user/WorkspaceStorage/w1/ChatSessions/Ab1.JSON"
	want := parsedRows(t, ".config/Code/User/workspaceStorage/w1/chatSessions/Ab1.json", session)
	got := parsedRows(t, respelled, session)
	if !New().Wants(&input.Artifact{Rel: respelled}) || len(want) == 0 || len(got) != len(want) || got[0].SessionID != "Ab1" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range got {
		got[i].SourceFile = want[i].SourceFile
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
