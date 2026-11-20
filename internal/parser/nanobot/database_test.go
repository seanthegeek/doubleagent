package nanobot

import (
	"fmt"
	"slices"
	"testing"
)

// A key is the sessions table's key, so a repeated one is damage: the
// key's messages are shown once, with the first row of the key in the
// order the rows are read, each later row gives its own start row, and the
// table one problem row, so the rows grow with the table rather than with
// the square of it.
func TestARepeatedSessionKeyShowsItsMessagesOnce(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d rows of one key and %d messages", count, count), func(t *testing.T) {
			got := rowsOfTableCaseDatabase(t, fmt.Sprintf("repeated_key_%d.db", count))
			const start = "session start: key=cli:direct"
			want := []string{start}
			for i := range count {
				want = append(want, fmt.Sprintf("message %d", i))
			}
			for range count - 1 {
				want = append(want, start)
			}
			want = append(want, fmt.Sprintf("parser: sessions holds %d rows repeating an earlier row's key; "+
				"its messages are shown once", count-1))
			if !slices.Equal(got, want) {
				t.Errorf("%d rows %q, want %d rows %q", len(got), got[:min(len(got), 4)], len(want), want[:4])
			}
		})
	}
}
