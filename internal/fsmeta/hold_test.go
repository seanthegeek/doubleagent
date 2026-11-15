package fsmeta

import (
	"errors"
	"testing"
)

// fakeFile is a file system's view of one file's access time, with the ways
// a file system may answer the hold: honour it, refuse it, or store the
// request as a literal time. Every restore is recorded.
type fakeFile struct {
	accessTime   uint64
	storesHold   bool
	refusesHold  bool
	failsRestore bool
	unreadable   bool
	restores     []uint64
}

// steps returns the operations verifyHold runs on the fake.
func (file *fakeFile) steps() holdSteps {
	return holdSteps{
		accessTime: func() (uint64, error) {
			if file.unreadable {
				return 0, errors.New("unreadable")
			}
			return file.accessTime, nil
		},
		hold: func() error {
			switch {
			case file.refusesHold:
				return errors.New("refused")
			case file.storesHold:
				file.accessTime = holdTime
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
	}
}

func TestVerifyHold(t *testing.T) {
	const original = 133_000_000_000_000_000
	cases := []struct {
		name         string
		file         fakeFile
		want         holdOutcome
		wantTime     uint64
		wantRestores int
	}{
		{"an honoured hold is not reported", fakeFile{accessTime: original},
			holdOutcome{}, original, 0},
		{"a refused hold is reported and the handle kept", fakeFile{accessTime: original,
			refusesHold: true}, holdOutcome{reason: reasonHoldRefused}, original, 0},
		{"a hold stored as a time is set back to the value read before it",
			fakeFile{accessTime: original, storesHold: true},
			holdOutcome{reason: reasonRestored, reopen: true}, original, 1},
		{"a hold stored as a time whose restore fails is reported as failed",
			fakeFile{accessTime: original, storesHold: true, failsRestore: true},
			holdOutcome{reason: reasonRestoreFailed, reopen: true}, holdTime, 1},
		{"an unreadable time keeps the hold, unverified",
			fakeFile{accessTime: original, storesHold: true, unreadable: true},
			holdOutcome{reason: reasonNotVerified}, holdTime, 0},
		{"a zero time read before is not sent back, since zero keeps the current time",
			fakeFile{accessTime: keepTime, storesHold: true},
			holdOutcome{reason: reasonRestoreFailed, reopen: true}, holdTime, 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			file := test.file
			got := verifyHold(file.steps())
			if got != test.want || file.accessTime != test.wantTime ||
				len(file.restores) != test.wantRestores {
				t.Errorf("outcome %+v, time %d, restores %v; want %+v, %d, %d restore(s)",
					got, file.accessTime, file.restores, test.want, test.wantTime, test.wantRestores)
			}
			for _, restored := range file.restores {
				if restored != original {
					t.Errorf("restored %d, want the value read before the hold, %d", restored, original)
				}
			}
		})
	}
}

func TestARestoreTheFileSystemDoesNotKeepIsReportedAsFailed(t *testing.T) {
	const original = 133_000_000_000_000_000
	file := fakeFile{accessTime: original, storesHold: true}
	steps := file.steps()
	steps.restore = func(uint64) error { return nil }
	got := verifyHold(steps)
	if want := (holdOutcome{reason: reasonRestoreFailed, reopen: true}); got != want {
		t.Errorf("outcome %+v, want %+v", got, want)
	}
}
