package openhands

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestATokenUsageModelIsCutOnceForEveryEventThatSwitchesToIt(t *testing.T) {
	const conversation = ".openhands/conversations/0123456789abcdef0123456789abcdef/"
	const events = 10
	first, second := strings.Repeat("a", 10000), strings.Repeat("b", 10000)
	files := map[string]string{conversation + "base_state.json": `{"stats": {"usage_to_metrics": {"agent": ` +
		`{"token_usages": [{"response_id": "r0", "model": "` + first + `"}, {"response_id": "r1", "model": "` +
		second + `"}]}}}}`}
	for i := range events {
		files[conversation+fmt.Sprintf("events/event-%05d-0123abcd.json", i)] = fmt.Sprintf(
			`{"kind": "MessageEvent", "llm_response_id": "r%d", "llm_message": {"role": "assistant", "content": "x"}}`,
			i%2)
	}
	home := writeTree(t, t.TempDir(), files)
	subject := New()
	var rows []timeline.Row
	for i := range events {
		rel := conversation + fmt.Sprintf("events/event-%05d-0123abcd.json", i)
		artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
			Rel: rel, Home: &input.Home{DiskPath: home, Original: "/h"}}
		// The run's own cut of long fields comes after the parse: the
		// models the rows carry are cut already, so it cuts none again.
		if _, err := parserkit.Collect(subject, artifact, parserkit.Options{}, func(row timeline.Row) {
			rows = append(rows, row)
		}); err != nil {
			t.Fatal(err)
		}
	}
	// A switch row holds two models, each cut, beside a few short words.
	const most = 2*parserkit.CarriedLimit + 64
	switches := 0
	for _, row := range rows {
		if len(row.Text) > most {
			t.Errorf("a row of %d bytes, more than %d: %.80q", len(row.Text), most, row.Text)
		}
		if strings.Contains(row.Text, " -> ") {
			switches++
		}
	}
	if switches != events-1 {
		t.Errorf("%d switch rows, want one per event after the first (%d)", switches, events-1)
	}
	want := []string{parserkit.ProblemPrefix + parserkit.CarriedProblem("token usage model", 2)}
	if got := problemTexts(rows); !slices.Equal(got, want) {
		t.Errorf("problems %q, want %q", got, want)
	}
}
