package cody

import (
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// The fixed parts of a path match in any ASCII case, as the catalog matches
// them; Cody takes no value from a path.
func TestAPathSpelledInAnotherCaseIsRead(t *testing.T) {
	const respelled = "CODY-NodeJS/jetbrains-GlobalState/Cody-Local-ChatHistory-V2"
	history := accountHistory("a", 1)
	want := parseSecrets(t, secrets.Setting{}, "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2", history)
	got := parseSecrets(t, secrets.Setting{}, respelled, history)
	if !New().Wants(&input.Artifact{Rel: respelled}) || len(want) == 0 || len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range got {
		got[i].SourceFile = want[i].SourceFile
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
