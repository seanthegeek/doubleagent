package stage

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

func TestAgainKeepsOnlyTheSameSourcesRecordedResult(t *testing.T) {
	source, other := [2]uint64{1, 10}, [2]uint64{1, 11}
	recorded := Copied{SHA256: "abc", Size: 3, ArchivePath: "fs/x"}
	cases := []struct {
		name     string
		staged   uint64 // the inode lstat gives the staged path
		failed   bool   // the lstat of the staged path fails
		source   [2]uint64
		want     bool
		recorded bool
	}{
		{"the same source again keeps its result", 7, false, source, true, true},
		{"another source is a collision", 7, false, other, false, true},
		{"an entry this run did not stage is refused", 8, false, source, false, true},
		{"nothing recorded at all is refused", 7, false, source, false, false},
		{"a staged entry that cannot be examined is refused", 7, true, source, false, true},
		{"a staged entry of inode 0 is refused", 0, false, source, false, true},
		{"a source of inode 0 is refused", 9, false, [2]uint64{1, 0}, false, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			places := NewPlaces()
			if testCase.recorded {
				places.record([2]uint64{1, 7}, source, "/s/fs/y", recorded)
				// Records an identity of 0 would match if it were trusted.
				places.record([2]uint64{1, 0}, source, "/s/fs/y", recorded)
				places.record([2]uint64{1, 9}, [2]uint64{1, 0}, "/s/fs/y", recorded)
			}
			stager := Stager{Dir: "/s", Places: places, stagedFS: &stagedFS{
				lstat: func(string) (fsmeta.Meta, error) {
					if testCase.failed {
						return fsmeta.Meta{}, errors.New("unreadable")
					}
					return fsmeta.Meta{Dev: 1, Ino: testCase.staged}, nil
				},
				list: func(string) ([]fs.DirEntry, error) { return nil, nil },
			}}
			got, err := stager.again("/src/x", "/s/fs/x", testCase.source)
			if testCase.want && (err != nil || got != recorded) || !testCase.want && err == nil {
				t.Errorf("again = %+v, %v; want the recorded result: %v", got, err, testCase.want)
			}
			// Only another source recorded at the place is that entry's name.
			wantCollision := testCase.name == "another source is a collision"
			if IsCaseCollision(err) != wantCollision {
				t.Errorf("again = %v; a collision: %v", err, wantCollision)
			}
		})
	}
}
