package fsmeta

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

// The access time the fake files start with, and a later one another
// reader could stamp, in the units of a FILETIME.
const (
	original = 133_000_000_000_000_000
	later    = original + 36_000_001_234
)

// fakeFile is a file system's view of one file's access time. onHold says
// what asking for the hold does to the stored time (nil: nothing, as an
// honoured hold), refusesHold makes the request fail after that, and
// failsRead lists the reads (from 0) that fail.
type fakeFile struct {
	accessTime  uint64
	onHold      func(uint64) uint64
	refusesHold bool
	failsRead   []int
	reads       int
}

// steps returns the operations verifyHold runs on the fake.
func (file *fakeFile) steps() holdSteps {
	return holdSteps{
		accessTime: func() (uint64, error) {
			file.reads++
			if slices.Contains(file.failsRead, file.reads-1) {
				return 0, errors.New("unreadable")
			}
			return file.accessTime, nil
		},
		hold: func() error {
			if file.onHold != nil {
				file.accessTime = file.onHold(file.accessTime)
			}
			if file.refusesHold {
				return errors.New("refused")
			}
			return nil
		},
	}
}

// secondReadings are what the second reading of the access time returns,
// by name: what asking for the hold does to the stored time, or a failed
// reading.
var secondReadings = map[string]func(file *fakeFile){
	"same":  func(*fakeFile) {},
	"later": func(file *fakeFile) { file.onHold = func(uint64) uint64 { return later } },
	"all ones": func(file *fakeFile) {
		file.onHold = func(uint64) uint64 { return ^uint64(0) }
	},
	"read failed": func(file *fakeFile) { file.failsRead = append(file.failsRead, 1) },
}

// The hold check only reports: no case sets the access time, so a time the
// file system misread from the request is reported as a change and kept.
func TestVerifyHold(t *testing.T) {
	// first: whether the first reading works; hold: whether the request is
	// accepted; second: secondReadings.
	cases := []struct {
		first, hold bool
		second      string
		want        string
	}{
		{true, true, "same", ""},
		{true, true, "later", reasonChanged},
		{true, true, "all ones", reasonChanged},
		{true, true, "read failed", reasonNotVerified},
		{true, false, "same", reasonHoldRefused},
		{true, false, "later", reasonChanged},
		{true, false, "all ones", reasonChanged},
		{true, false, "read failed", reasonHoldRefused},
		{false, true, "same", reasonNotVerified},
		{false, true, "all ones", reasonNotVerified},
		{false, false, "all ones", reasonHoldRefused},
	}
	for _, test := range cases {
		name := fmt.Sprintf("first reading ok %v, hold accepted %v, second reading %s",
			test.first, test.hold, test.second)
		t.Run(name, func(t *testing.T) {
			file := fakeFile{accessTime: original, refusesHold: !test.hold}
			if !test.first {
				file.failsRead = []int{0}
			}
			secondReadings[test.second](&file)
			if got := verifyHold(file.steps()); got != test.want || file.reads != 2 {
				t.Errorf("reason %q after %d reads; want %q after 2", got, file.reads, test.want)
			}
		})
	}
}

func TestReportsOnVolume(t *testing.T) {
	cases := []struct {
		name     string
		reason   string
		readOnly bool
		want     bool
	}{
		{"a refused right on a writable volume is reported", reasonRightRefused, false, true},
		{"a refused right on a read-only volume is not", reasonRightRefused, true, false},
		{"a refused hold on a read-only volume is not", reasonHoldRefused, true, false},
		{"a changed time on a read-only volume is still reported", reasonChanged, true, true},
		{"a hold in place is not reported", "", false, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := reportsOnVolume(test.reason, func() bool { return test.readOnly }); got != test.want {
				t.Errorf("reportsOnVolume = %v, want %v", got, test.want)
			}
		})
	}
}
