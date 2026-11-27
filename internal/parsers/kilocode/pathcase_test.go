package kilocode

import (
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// The fixed parts of a path match in any ASCII case, as the catalog matches
// them, and the task id keeps the spelling the disk gave.
func TestAPathSpelledInAnotherCaseIsRead(t *testing.T) {
	const task = `[{"role": "user", "content": "hi", "ts": 1791021600000}]`
	const respelled = ".config/Code/USER/GlobalStorage/KiloCode.Kilo-Code/Tasks/Ab1/API_Conversation_History.JSON"
	want := parseSecrets(t, secrets.Setting{},
		".config/Code/User/globalStorage/kilocode.kilo-code/tasks/Ab1/api_conversation_history.json", task)
	got := parseSecrets(t, secrets.Setting{}, respelled, task)
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
