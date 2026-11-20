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

func TestCutAheadLeavesRoomForASuffixAfterTheValue(t *testing.T) {
	const room = 21
	long := filler(9000)
	tests := []struct {
		name, value, want string
	}{
		{"a value that fits with the room is written whole", filler(CarriedLimit - room),
			filler(CarriedLimit - room)},
		{"a value one byte too long for the room is cut", filler(CarriedLimit - room + 1),
			filler(CarriedLimit - room + 1)[:CarriedLimit-room-len(cutMark(CarriedLimit-room+1))] +
				cutMark(CarriedLimit-room+1)},
		{"a long value is cut to leave the room", long, long[:CarriedLimit-room-len(cutMark(9000))] + cutMark(9000)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := NewCarried(&input.Artifact{}).CutAhead("chat id", test.value, room)
			if got != test.want || len(got)+room > CarriedLimit {
				t.Errorf("CutAhead gave %d bytes, want %d within %d", len(got), len(test.want), CarriedLimit-room)
			}
		})
	}
}

// A value cut ahead of many suffixes is cut once and given to the secrets
// recorder once, and its cuts are counted for one problem.
func TestCutAheadCutsAValueGivenAgainOnce(t *testing.T) {
	work := CountingWork()
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true})
	recorder.CountWork(work)
	carried := NewCarried(&input.Artifact{Home: &input.Home{Secrets: recorder}})
	long := filler(9000)
	for range 100 {
		carried.CutAhead("chat id", long, 21)
	}
	var problems Problems
	carried.Report(0, &problems)
	want := Problems{{Message: CarriedProblem("chat id", 100)}}
	if compared := work.Count("cut bytes compared"); compared != 0 || len(recorder.CutValues()) != 1 ||
		!slices.Equal(problems, want) {
		t.Errorf("compared %d bytes, recorded %d cuts and problems %v; want none, one and %v", compared,
			len(recorder.CutValues()), problems, want)
	}
}
