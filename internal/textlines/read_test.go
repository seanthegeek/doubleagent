package textlines

import (
	"slices"
	"strings"
	"testing"
)

func TestReadLinesBoundsEachLine(t *testing.T) {
	type line struct {
		number int
		text   string
		long   bool
	}
	tests := []struct {
		name, data string
		limit      int
		want       []line
	}{
		{"lines are numbered and lose their LF", "a\n\nb", 4,
			[]line{{1, "a", false}, {2, "", false}, {3, "b", false}}},
		{"a line of exactly the bound is kept", "abcd\nx\n", 4,
			[]line{{1, "abcd", false}, {2, "x", false}}},
		{"a longer line is one long line and the rest is read", "abcde\nx\n", 4,
			[]line{{1, "", true}, {2, "x", false}}},
		{"a long last line without an LF is long", "x\nabcde", 4,
			[]line{{1, "x", false}, {2, "", true}}},
		{"a line longer than the reader's buffer is long", strings.Repeat("z", 200000) + "\nok\n", 100000,
			[]line{{1, "", true}, {2, "ok", false}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []line
			err := ReadLines(t.Context(), strings.NewReader(test.data), test.limit,
				func(number int, text []byte, long bool) { got = append(got, line{number, string(text), long}) })
			if err != nil || !slices.Equal(got, test.want) {
				t.Errorf("got %v (%v), want %v", got, err, test.want)
			}
		})
	}
}
