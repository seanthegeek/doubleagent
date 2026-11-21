package tabby

import (
	"fmt"
	"slices"
	"testing"
)

// A thread or page id is its table's key, so a repeated one is damage: the
// thread's messages and the page's sections are shown once, with the first
// row of the id in the order the rows are read, and each later row gives
// its start row alone and adds to one problem row for its table, so the
// rows grow with the table rather than with the square of it. The id 1
// and the text '1' are two ids, as SQL's = finds them in a column of no
// declared type (sqlite3: SELECT 1 = '1' between two such columns gives
// 0), so the last thread row, whose id is the text, is a thread of its own
// with no message and no repeat.
func TestARepeatedThreadOrPageIDShowsItsChildrenOnce(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d rows of each id", count), func(t *testing.T) {
			got := parseDatabase(t, fmt.Sprintf("repeated_ids_%d.db", count))
			want := []string{"thread start"}
			for i := range count {
				want = append(want, fmt.Sprintf("message %d", i))
			}
			for range count - 1 {
				want = append(want, "thread start")
			}
			want = append(want, "page start | title copy 0")
			for i := range count {
				want = append(want, fmt.Sprintf("section %d\n\nbody", i))
			}
			for i := 1; i < count; i++ {
				want = append(want, fmt.Sprintf("page start | title copy %d", i))
			}
			want = append(want,
				fmt.Sprintf("parser: threads holds %d rows repeating an earlier row's id; "+
					"its messages are shown once", count-2),
				fmt.Sprintf("parser: pages holds %d rows repeating an earlier row's id; "+
					"its sections are shown once", count-1))
			if !slices.Equal(got, want) {
				t.Errorf("%d rows ending %q, want %d rows ending %q", len(got), got[max(0, len(got)-3):],
					len(want), want[len(want)-3:])
			}
		})
	}
}
