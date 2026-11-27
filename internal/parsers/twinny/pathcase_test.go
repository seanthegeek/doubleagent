package twinny

import (
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The fixed parts of a path match in any ASCII case, as the catalog matches
// them, and the log's name in row text keeps the spelling the disk gave.
func TestAPathSpelledInAnotherCaseIsRead(t *testing.T) {
	const log = "2026-10-01 08:00:00.000 [info] started\n"
	rowsAt := func(rel string) (bool, []timeline.Row) {
		artifact := writeArtifact(t, rel, log)
		var rows []timeline.Row
		if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
			row.SourceFile = ""
			rows = append(rows, row)
		}); err != nil {
			t.Fatal(err)
		}
		return New().Wants(artifact), rows
	}
	_, want := rowsAt("logs/20261001/window1/exthost/rjmacarthy.twinny/Twinny.log")
	wanted, got := rowsAt("LOGS/20261001/window1/exthost/RJMacArthy.Twinny/TWINNY.LOG")
	for i := range want {
		want[i].Text = strings.ReplaceAll(want[i].Text, "Twinny.log", "TWINNY.LOG")
	}
	if !wanted || len(want) == 0 || !reflect.DeepEqual(got, want) {
		t.Errorf("wanted %v, got %+v, want %+v", wanted, got, want)
	}
}
