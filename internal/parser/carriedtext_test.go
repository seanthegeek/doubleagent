package parser

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
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
