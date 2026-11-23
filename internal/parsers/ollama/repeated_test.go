package ollama

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A message id is the key of the messages table, so a repeated one is
// damage: the id's tool calls and attachments are shown once, with the
// first message of the id in the order the rows are read, and each later
// message gives its own rows without them and adds to one problem row, so
// the rows grow with the table rather than with the square of it. Two
// values SQL's = finds equal, 3 and 3.0, are one id.
func TestARepeatedMessageIDShowsItsCallsAndAttachmentsOnce(t *testing.T) {
	tests := []struct {
		name, database string
		want           []string
	}{
		{"10 rows of each id give each call and attachment once", "repeated_message_id_10.db",
			repeatedRows(10)},
		{"40 rows of each id give each call and attachment once", "repeated_message_id_40.db",
			repeatedRows(40)},
		{"an id stored as 3 and as 3.0 gives its call once", "equal_message_id.db",
			[]string{"model: llama3", "first", "q", "second",
				"parser: messages holds 1 row repeating an earlier row's id; " +
					"its tool calls and attachments are shown once"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := rowsOfTableCaseDatabase(t, test.database)
			if !slices.Equal(got, test.want) {
				t.Errorf("%d rows ending %q, want %d rows ending %q", len(got), got[max(0, len(got)-3):],
					len(test.want), test.want[len(test.want)-3:])
			}
		})
	}
}

// repeatedRows returns the rows of repeated_message_id_<count>.db: the
// first assistant message with every call, the later ones with their own
// text alone, the first user message with every attachment, the later ones
// alone, and the problem row counting the repeats of both ids.
func repeatedRows(count int) []string {
	want := []string{"model: llama3", "answer 0"}
	for i := range count {
		want = append(want, fmt.Sprintf("q%d", i))
	}
	for i := 1; i < count; i++ {
		want = append(want, fmt.Sprintf("answer %d", i))
	}
	first := []string{"question 0"}
	for i := range count {
		first = append(first, fmt.Sprintf("[attachment: f%d.txt, 1 bytes]", i))
	}
	want = append(want, strings.Join(first, "\n"))
	for i := 1; i < count; i++ {
		want = append(want, fmt.Sprintf("question %d", i))
	}
	return append(want, fmt.Sprintf("parser: messages holds %d rows repeating an earlier row's id; "+
		"its tool calls and attachments are shown once", 2*(count-1)))
}
