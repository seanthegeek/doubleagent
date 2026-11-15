package fsmeta

import (
	"errors"
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

func TestVerifyHold(t *testing.T) {
	cases := []struct {
		name     string
		file     fakeFile
		want     string
		wantTime uint64
	}{
		{"an honoured hold is not reported", fakeFile{}, "", original},
		{"a refused hold is reported", fakeFile{refusesHold: true}, reasonHoldRefused, original},
		{"another reader moving the time forward is kept and reported",
			fakeFile{onHold: func(uint64) uint64 { return fakeNow }}, reasonChanged, fakeNow},
		{"a second read at a coarser grain is the same time, not reported",
			fakeFile{onHold: func(time uint64) uint64 { return time - time%grainAllowance }}, "",
			original - original%grainAllowance},
		{"the hold value stored as a time is set back",
			fakeFile{onHold: storesHold}, reasonRestored, original},
		{"a time far in the future is set back",
			fakeFile{onHold: func(uint64) uint64 { return fakeNow + 2*futureAllowance }},
			reasonRestored, original},
		{"a time before the one read first, beyond the grain, is set back",
			fakeFile{onHold: func(uint64) uint64 { return 1 }}, reasonRestored, original},
		{"a refused hold that stored the value is set back",
			fakeFile{onHold: storesHold, refusesHold: true}, reasonRestored, original},
		{"a stored hold whose restore fails is reported as failed",
			fakeFile{onHold: storesHold, failsRestore: true}, reasonRestoreFailed, holdTime},
		{"a failed first read with the hold value after is a failed restore",
			fakeFile{onHold: storesHold, failsRead: []int{0}}, reasonRestoreFailed, holdTime},
		{"a failed first read with an ordinary time after is unverified",
			fakeFile{failsRead: []int{0}}, reasonNotVerified, original},
		{"a failed second read is unverified", fakeFile{onHold: storesHold, failsRead: []int{1}},
			reasonNotVerified, holdTime},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			file := test.file
			file.accessTime = original
			got := verifyHold(file.steps())
			if got != test.want || file.accessTime != test.wantTime {
				t.Errorf("reason %q, time %d; want %q, %d", got, file.accessTime, test.want,
					test.wantTime)
			}
			restored := test.want == reasonRestored ||
				(test.want == reasonRestoreFailed && file.failsRestore)
			if (len(file.restores) > 0) != restored {
				t.Errorf("restores %v, want one only when the time is set back", file.restores)
			}
			for _, value := range file.restores {
				if value != original {
					t.Errorf("restored %d, want the value read before the hold, %d", value, original)
				}
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
