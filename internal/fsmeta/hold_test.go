package fsmeta

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

// The fake clock and the access time the fake files start with, in the
// units of a FILETIME: a recent time, and an hour before it, off any grain.
const (
	fakeNow  = 133_000_000_000_000_000
	original = fakeNow - 36_000_001_234
)

// fakeFile is a file system's view of one file's access time. onHold says
// what asking for the hold does to the stored time (nil: nothing, as an
// honoured hold), refusesHold makes the request fail after that, failsRead
// lists the reads (from 0) that fail, and every restore is recorded.
type fakeFile struct {
	accessTime   uint64
	onHold       func(uint64) uint64
	refusesHold  bool
	failsRestore bool
	failsRead    []int
	reads        int
	restores     []uint64
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
		restore: func(accessTime uint64) error {
			file.restores = append(file.restores, accessTime)
			if file.failsRestore {
				return errors.New("restore refused")
			}
			file.accessTime = accessTime
			return nil
		},
		now: func() uint64 { return fakeNow },
	}
}

// storesHold is a file system that stores the hold request as a time.
func storesHold(uint64) uint64 { return holdTime }

// secondReadings are what the second reading of the access time returns,
// by name: what asking for the hold does to the stored time, or a failed
// reading.
var secondReadings = map[string]func(file *fakeFile){
	"same":        func(*fakeFile) {},
	"forward":     func(file *fakeFile) { file.onHold = func(uint64) uint64 { return fakeNow } },
	"far future":  func(file *fakeFile) { file.onHold = func(uint64) uint64 { return fakeNow + 2*futureAllowance } },
	"all ones":    func(file *fakeFile) { file.onHold = storesHold },
	"backwards":   func(file *fakeFile) { file.onHold = func(uint64) uint64 { return 1 } },
	"read failed": func(file *fakeFile) { file.failsRead = append(file.failsRead, 1) },
}

func TestVerifyHold(t *testing.T) {
	// first: whether the first reading works; hold: whether the request is
	// accepted; second: secondReadings; restores: whether a restore is sent.
	cases := []struct {
		first, hold bool
		second      string
		want        string
		restores    bool
	}{
		{true, true, "same", "", false},
		{true, true, "forward", reasonChanged, false},
		{true, true, "far future", reasonRestored, true},
		{true, true, "all ones", reasonRestored, true},
		{true, true, "backwards", reasonRestored, true},
		{true, true, "read failed", reasonNotVerified, false},
		{true, false, "same", reasonHoldRefused, false},
		{true, false, "forward", reasonHoldRefused, false},
		{true, false, "far future", reasonRestored, true},
		{true, false, "all ones", reasonRestored, true},
		{true, false, "backwards", reasonRestored, true},
		{true, false, "read failed", reasonHoldRefused, false},
		{false, true, "same", reasonNotVerified, false},
		{false, true, "forward", reasonNotVerified, false},
		{false, true, "far future", reasonNotVerified, false},
		{false, true, "all ones", reasonRestoreFailed, false},
		{false, true, "backwards", reasonNotVerified, false},
		{false, true, "read failed", reasonNotVerified, false},
		{false, false, "same", reasonHoldRefused, false},
		{false, false, "forward", reasonHoldRefused, false},
		{false, false, "far future", reasonHoldRefused, false},
		{false, false, "all ones", reasonRestoreFailed, false},
		{false, false, "backwards", reasonHoldRefused, false},
		{false, false, "read failed", reasonHoldRefused, false},
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
			got := verifyHold(file.steps())
			if got != test.want || (len(file.restores) > 0) != test.restores {
				t.Errorf("reason %q, restores %v; want %q, a restore sent %v", got, file.restores,
					test.want, test.restores)
			}
			for _, value := range file.restores {
				if value != original {
					t.Errorf("restored %d, want the value read before the hold, %d", value, original)
				}
			}
		})
	}
}

func TestVerifyHoldEdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		file     fakeFile
		want     string
		wantTime uint64
		restores int
	}{
		{"a time already far in the future, unchanged, is left alone",
			fakeFile{accessTime: fakeNow + 2*futureAllowance}, "", fakeNow + 2*futureAllowance, 0},
		{"a time already the hold value, unchanged, is left alone",
			fakeFile{accessTime: holdTime, onHold: storesHold}, "", holdTime, 0},
		{"a second reading at a coarser grain is the same time, not reported",
			fakeFile{accessTime: original,
				onHold: func(time uint64) uint64 { return time - time%grainAllowance }},
			"", original - original%grainAllowance, 0},
		{"a stored hold whose restore fails is reported as failed",
			fakeFile{accessTime: original, onHold: storesHold, failsRestore: true},
			reasonRestoreFailed, holdTime, 1},
		{"a restored time is the value read before the hold",
			fakeFile{accessTime: original, onHold: storesHold}, reasonRestored, original, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			file := test.file
			got := verifyHold(file.steps())
			if got != test.want || file.accessTime != test.wantTime ||
				len(file.restores) != test.restores {
				t.Errorf("reason %q, time %d, restores %v; want %q, %d, %d restore(s)", got,
					file.accessTime, file.restores, test.want, test.wantTime, test.restores)
			}
		})
	}
}

func TestARestoreThatCannotBeConfirmedIsReportedAsFailed(t *testing.T) {
	cases := []struct {
		name   string
		before uint64
		keeps  bool
	}{
		{"a restore the file system does not keep", original, false},
		{"a zero time read before, which a set would take as keep the current time", keepTime, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			file := fakeFile{accessTime: test.before, onHold: storesHold}
			steps := file.steps()
			sent := 0
			steps.restore = func(uint64) error { sent++; return nil }
			if got := verifyHold(steps); got != reasonRestoreFailed {
				t.Errorf("reason %q, want %q", got, reasonRestoreFailed)
			}
			if test.keeps && sent != 0 {
				t.Errorf("a value the file system takes as a request was sent")
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
		{"a restore on a read-only volume is still reported", reasonRestored, true, true},
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
