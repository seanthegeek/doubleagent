package openhands

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func FuzzNaive(fuzz *testing.F) {
	for _, seed := range []string{"", "2026-10-01T08:00:00.123456", "2026-10-01", "2026-10-01T24:00:00", "9999-12-31T24:00:00"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		naive(jsonvalue.NewString(text))
	})
}
