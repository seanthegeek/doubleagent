package pearai

import (
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
)

// The fixed parts of a path match in any ASCII case, as the catalog matches
// them, and the session id taken from the file name keeps the disk's
// spelling.
func TestAPathSpelledInAnotherCaseIsRead(t *testing.T) {
	const session = `{"title": "t", "history": [{"message": {"role": "user", "content": "hi"}}]}`
	const respelled = ".PEARAI/Sessions/Ab1.JSON"
	want := parsertest.RowsAt(t, New(), ".pearai/sessions/Ab1.json", session)
	got := parsertest.RowsAt(t, New(), respelled, session)
	for i := range min(len(got), len(want)) {
		got[i].SourceFile = want[i].SourceFile
	}
	wanted := New().Wants(&input.Artifact{Rel: respelled})
	if !wanted || len(want) == 0 || !reflect.DeepEqual(got, want) || got[0].SessionID != "Ab1" {
		t.Errorf("wanted %v, got %+v, want %+v", wanted, got, want)
	}
}
