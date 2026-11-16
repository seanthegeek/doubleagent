package follow

import (
	"errors"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

func TestSameIdentityNeedsTheFileSystemsWord(t *testing.T) {
	metas := map[string]fsmeta.Meta{
		"C:/u/Alice": {Dev: 7, Ino: 100}, "C:/u/ALICE": {Dev: 7, Ino: 100},
		"C:/u/alice": {Dev: 7, Ino: 200}, "C:/u/Busy": {}, "C:/u/BUSY": {},
		"C:/v/Alice": {Dev: 8, Ino: 100}, "C:/out/Alice": {Dev: 7, Ino: 100},
	}
	lstat := func(path string) (fsmeta.Meta, error) {
		meta, found := metas[path]
		if !found {
			return fsmeta.Meta{}, errors.New("gone")
		}
		return meta, nil
	}
	check := func(path string) (Refusal, bool) {
		return Refusal{Link: path, Kind: LeadsOut}, path == "C:/out/Alice"
	}
	cases := []struct {
		name, first, second string
		want                bool
	}{
		{"two spellings with one identity are one entry", "C:/u/Alice", "C:/u/ALICE", true},
		{"two identities are two entries", "C:/u/Alice", "C:/u/alice", false},
		{"one that cannot be examined proves nothing", "C:/u/Alice", "C:/u/aLICE", false},
		{"zero identities prove nothing", "C:/u/Busy", "C:/u/BUSY", false},
		{"one file index on two volumes is two entries", "C:/u/Alice", "C:/v/Alice", false},
		{"a path the root check refuses is not looked at", "C:/u/Alice", "C:/out/Alice", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := sameIdentity(check, lstat, testCase.first, testCase.second); got != testCase.want {
				t.Errorf("sameIdentity(%s, %s) = %v, want %v", testCase.first, testCase.second,
					got, testCase.want)
			}
		})
	}
}
