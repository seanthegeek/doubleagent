package fsmeta

import (
	"errors"
	"io/fs"
	"testing"
)

func TestSameEntryNeedsTheFileSystemsWord(t *testing.T) {
	metas := map[string]Meta{
		"C:/u/Alice": {Dev: 7, Ino: 100}, "C:/u/ALICE": {Dev: 7, Ino: 100},
		"C:/u/alice": {Dev: 7, Ino: 200}, "C:/u/Busy": {}, "C:/u/BUSY": {},
		"C:/v/Alice": {Dev: 8, Ino: 100},
	}
	denied := errors.New("access is denied")
	lstat := func(path string) (Meta, error) {
		if meta, found := metas[path]; found {
			return meta, nil
		}
		if path == "C:/u/aLICE" {
			return Meta{}, &fs.PathError{Op: "lstat", Path: path, Err: denied}
		}
		return Meta{}, &fs.PathError{Op: "lstat", Path: path, Err: fs.ErrNotExist}
	}
	cases := []struct {
		name, first, second string
		want                bool
	}{
		{"two spellings with one identity are one entry", "C:/u/Alice", "C:/u/ALICE", true},
		{"two identities are two entries", "C:/u/Alice", "C:/u/alice", false},
		{"a spelling that does not exist joins the other", "C:/u/Alice", "C:/u/aLiCe", true},
		{"two spellings that do not exist are one", "C:/gone/X", "C:/gone/x", true},
		{"one that cannot be examined for another reason proves nothing",
			"C:/u/Alice", "C:/u/aLICE", false},
		{"zero identities prove nothing", "C:/u/Busy", "C:/u/BUSY", false},
		{"one file index on two volumes is two entries", "C:/u/Alice", "C:/v/Alice", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := sameEntry(lstat, testCase.first, testCase.second); got != testCase.want {
				t.Errorf("sameEntry(%s, %s) = %v, want %v", testCase.first, testCase.second,
					got, testCase.want)
			}
		})
	}
}
