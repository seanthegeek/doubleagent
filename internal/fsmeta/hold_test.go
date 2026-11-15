package fsmeta

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

// The fake clock and the access time the fake files start with, in the
// units of a FILETIME: a recent time, and an hour before it; and spans of
// ten minutes, two days and sixty years in the same units.
const (
	fakeNow    = 133_000_000_000_000_000
	original   = fakeNow - 36_000_001_234
	tenMinutes = 10 * 60 * 10_000_000
	twoDays    = 2 * 24 * 60 * 60 * 10_000_000
	sixtyYears = 60 * 36525 * 24 * 60 * 60 * 10_000_000 / 100
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
	"same":           func(*fakeFile) {},
	"now":            func(file *fakeFile) { file.onHold = func(uint64) uint64 { return fakeNow } },
	"60 years ahead": func(file *fakeFile) { file.onHold = func(uint64) uint64 { return fakeNow + sixtyYears } },
	"all ones":       func(file *fakeFile) { file.onHold = storesHold },
	"before 1980":    func(file *fakeFile) { file.onHold = func(uint64) uint64 { return floorTime - 1 } },
	"read failed":    func(file *fakeFile) { file.failsRead = append(file.failsRead, 1) },
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
		{true, true, "now", reasonChanged, false},
		{true, true, "60 years ahead", reasonRestored, true},
		{true, true, "all ones", reasonRestored, true},
		{true, true, "before 1980", reasonRestored, true},
		{true, true, "read failed", reasonNotVerified, false},
		{true, false, "same", reasonHoldRefused, false},
		{true, false, "now", reasonHoldRefused, false},
		{true, false, "60 years ahead", reasonRestored, true},
		{true, false, "all ones", reasonRestored, true},
		{true, false, "before 1980", reasonRestored, true},
		{true, false, "read failed", reasonHoldRefused, false},
		{false, true, "same", reasonNotVerified, false},
		{false, true, "now", reasonNotVerified, false},
		{false, true, "60 years ahead", reasonNotVerified, false},
		{false, true, "all ones", reasonRestoreFailed, false},
		{false, true, "before 1980", reasonNotVerified, false},
		{false, true, "read failed", reasonNotVerified, false},
		{false, false, "same", reasonHoldRefused, false},
		{false, false, "now", reasonHoldRefused, false},
		{false, false, "60 years ahead", reasonHoldRefused, false},
		{false, false, "all ones", reasonRestoreFailed, false},
		{false, false, "before 1980", reasonHoldRefused, false},
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

func TestVerifyHoldJudgesTheSecondReadingOnItsOwn(t *testing.T) {
	// before and after are the two readings of a hold that is accepted.
	cases := []struct {
		name          string
		before, after uint64
		want          string
		restores      bool
	}{
		{"a first reading 10 minutes ahead, then now, is another reader's",
			fakeNow + tenMinutes, fakeNow, reasonChanged, false},
		{"a first reading 2 days ahead, then now, is another reader's",
			fakeNow + twoDays, fakeNow, reasonChanged, false},
		{"a first reading 60 years ahead, then now, is another reader's",
			fakeNow + sixtyYears, fakeNow, reasonChanged, false},
		{"a second reading 2 days ahead is another reader's", original, fakeNow + twoDays,
			reasonChanged, false},
		{"a second reading a second behind is another reader's", original, original - 10_000_000,
			reasonChanged, false},
		{"a second reading just before 1980 is a misread", original, floorTime - 1,
			reasonRestored, true},
		{"a second reading on 1980-01-01 is possible", original, floorTime, reasonChanged, false},
		{"a time already 60 years ahead, unchanged, is left alone", fakeNow + sixtyYears,
			fakeNow + sixtyYears, "", false},
		{"a time already the hold value, unchanged, is left alone", holdTime, holdTime, "", false},
		{"a first reading of the keep value, then now, is another reader's", keepTime, fakeNow,
			reasonChanged, false},
		{"a first reading of the hold value, then now, is another reader's", holdTime, fakeNow,
			reasonChanged, false},
		{"a first reading of the resume value, then now, is another reader's", resumeTime, fakeNow,
			reasonChanged, false},
		{"a first reading of the keep value, then the hold value, cannot be restored", keepTime,
			holdTime, reasonRestoreFailed, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			after := test.after
			file := fakeFile{accessTime: test.before, onHold: func(uint64) uint64 { return after }}
			got := verifyHold(file.steps())
			if got != test.want || (len(file.restores) > 0) != test.restores {
				t.Errorf("reason %q, restores %v; want %q, a restore sent %v", got, file.restores,
					test.want, test.restores)
			}
			if test.restores && file.accessTime != test.before {
				t.Errorf("time %d after the restore, want %d", file.accessTime, test.before)
			}
		})
	}
}

func TestAFailingRestoreIsReported(t *testing.T) {
	file := fakeFile{accessTime: original, onHold: storesHold, failsRestore: true}
	if got := verifyHold(file.steps()); got != reasonRestoreFailed || len(file.restores) != 1 {
		t.Errorf("reason %q, restores %v; want %q after one restore", got, file.restores,
			reasonRestoreFailed)
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
