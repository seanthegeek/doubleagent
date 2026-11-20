package parser

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

func TestCarriedCutsOnlyLongValuesAndReportsOnceAName(t *testing.T) {
	long := filler(9000)
	carried := NewCarried(&input.Artifact{})
	var texts []string
	for _, value := range []string{"short", long, long, "other"} {
		texts = append(texts, carried.Cut("settings key", value))
	}
	texts = append(texts, carried.Cut("group name", long))
	var problems Problems
	carried.Report(7, &problems)
	want := Problems{{Line: 7, Message: "settings key longer than 4096 bytes, cut to 4096 bytes (2 times)"},
		{Line: 7, Message: "group name longer than 4096 bytes, cut to 4096 bytes (1 time)"}}
	cut := long[:CarriedLimit-len(cutMark(9000))] + cutMark(9000)
	if !slices.Equal(texts, []string{"short", cut, cut, "other", cut}) || !slices.Equal(problems, want) {
		t.Errorf("texts of %d bytes and problems %v, want %v", len(texts[1]), problems, want)
	}
	problems = nil
	carried.Report(8, &problems)
	if len(problems) != 0 {
		t.Errorf("a second report gave %v", problems)
	}
}

// Two long values cut in turn under two names are each cut once: the
// cut of each name is kept, so neither is read again for the secrets
// recorder however often the names take turns.
func TestCarriedCutsEachNamesValueOnceWhenNamesTakeTurns(t *testing.T) {
	work := CountingWork()
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true})
	recorder.CountWork(work)
	carried := NewCarried(&input.Artifact{Home: &input.Home{Secrets: recorder}})
	first, second := filler(9000), "/other"+filler(9000)
	for range 100 {
		carried.Cut("sandbox", first)
		carried.Cut("network", second)
	}
	if compared := work.Count("cut bytes compared"); compared != 0 {
		t.Errorf("the recorder compared %d bytes of cut values, want none", compared)
	}
}
