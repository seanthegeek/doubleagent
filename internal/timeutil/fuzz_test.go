package timeutil

import (
	"testing"
)

// FuzzFromText checks that any text gives "" or a timestamp in the timeline
// form, which EpochMillis reads back, without a panic.
func FuzzFromText(fuzz *testing.F) {
	for _, test := range textCases() {
		fuzz.Add(test.input)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		got := FromText(text)
		if got == "" {
			return
		}
		if _, ok := EpochMillis(got); !ok {
			t.Fatalf("FromText(%q) = %q, which is not in the timeline form", text, got)
		}
	})
}
