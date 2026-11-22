package sqlitefile

import (
	"bytes"
	"testing"
)

func TestMainViewLaysJournalPagesOverTheFile(t *testing.T) {
	file := []byte("abcdefgh")
	journal := []byte("..WXYZ..")
	cases := []struct {
		name     string
		size     int64
		restored map[int64]int64
		offset   int64
		length   int
		want     string
	}{
		{"a restored chunk replaces the file's bytes", 8, map[int64]int64{1: 2}, 0, 8, "abcdWXYZ"},
		{"a read across chunks joins file and journal", 8, map[int64]int64{1: 2}, 2, 4, "cdWX"},
		{"bytes past the file and inside the size are zero", 10, nil, 6, 4, "gh\x00\x00"},
		{"a restored chunk past the file's end is read", 12, map[int64]int64{2: 2}, 8, 4, "WXYZ"},
		{"bytes past a cut are zero", 5, nil, 3, 4, "de\x00\x00"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			view := newMainView(bytes.NewReader(file), int64(len(file)))
			view.size, view.chunk, view.restored = test.size, 4, test.restored
			view.journal = bytes.NewReader(journal)
			target := bytes.Repeat([]byte{'?'}, test.length)
			if err := view.readAt(target, test.offset); err != nil {
				t.Fatal(err)
			}
			if string(target) != test.want {
				t.Errorf("readAt(%d) = %q, want %q", test.offset, target, test.want)
			}
		})
	}
}

func TestJournalChecksumSamplesEvery200thByte(t *testing.T) {
	page := make([]byte, 1024)
	for _, offset := range []int{824, 624, 424, 224, 24} {
		page[offset] = 1
	}
	page[0], page[1023] = 100, 100
	if got := journalChecksum(page, 7); got != 12 {
		t.Errorf("journalChecksum() = %d, want 12", got)
	}
}
