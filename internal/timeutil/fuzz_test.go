package timeutil

import (
	"testing"
	"time"
)

// FuzzFromText checks that any text gives "" or a timestamp in the timeline
// form, which EpochMillis reads back, without a panic.
func FuzzFromText(fuzz *testing.F) {
	for _, test := range append(acceptedText(), refusedText()...) {
		fuzz.Add(test[0])
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

// FuzzEpochMillis checks that EpochMillis accepts a text only when
// FormatUTC writes that text back from the time it read.
func FuzzEpochMillis(fuzz *testing.F) {
	for _, seed := range []string{"2026-10-01T08:00:00.123Z", "0001-01-01T00:00:00.000Z",
		"9999-12-31T23:59:59.999Z", "6000-04-22T08:57:12.+40Z", "2026-10-01T08:00:00,123Z", ""} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		millis, ok := EpochMillis(text)
		if ok && FormatUTC(time.UnixMilli(millis)) != text {
			t.Fatalf("EpochMillis(%q) = %d, which FormatUTC writes as %q", text, millis,
				FormatUTC(time.UnixMilli(millis)))
		}
	})
}
