package fsmeta

// The reasons Opener.Report receives for a file read without the hold on
// its access time that the run wanted.
const (
	reasonRightRefused  = "right to hold refused"
	reasonHoldRefused   = "hold refused"
	reasonNotVerified   = "hold not verified"
	reasonRestored      = "hold not honoured, access time restored"
	reasonRestoreFailed = "hold not honoured, restore failed"
)

// Access times, as the 64-bit count of 100-nanosecond intervals since 1601
// a FILETIME holds, that a file system takes as a request rather than as a
// time when they are set: 0 keeps the current time, all ones asks that
// I/O through the handle not update the time (the hold), and all ones less
// one asks that updates resume.
const (
	keepTime   = 0
	holdTime   = ^uint64(0)
	resumeTime = ^uint64(0) - 1
)

// holdSteps are the operations verifyHold runs on one open file: read its
// access time, ask that reads not update it, and set it to a given value
// (and no other time).
type holdSteps struct {
	accessTime func() (uint64, error)
	hold       func() error
	restore    func(accessTime uint64) error
}

// holdOutcome is what verifyHold found: the reason to report ("" when the
// hold is in place), and whether the handle must be closed and the file
// opened for reading only.
type holdOutcome struct {
	reason string
	reopen bool
}

// verifyHold asks for the hold and reads the access time before and after
// it. A file system that stored the hold request as a literal time has
// overwritten the file's access time; the value read before is then set
// back, and the file is to be read on a new read-only handle, so nothing
// more is sent to a file system that misread the first request. When
// either read fails the hold is kept, unverified.
func verifyHold(steps holdSteps) holdOutcome {
	before, beforeErr := steps.accessTime()
	if steps.hold() != nil {
		return holdOutcome{reason: reasonHoldRefused}
	}
	after, afterErr := steps.accessTime()
	switch {
	case beforeErr != nil || afterErr != nil:
		return holdOutcome{reason: reasonNotVerified}
	case after == before:
		return holdOutcome{}
	}
	return holdOutcome{reason: restoreAccessTime(steps, before), reopen: true}
}

// restoreAccessTime sets the access time back to before, the value read on
// the same handle before the hold, and reads it again to confirm. A value
// the file system would take as a request (keepTime, holdTime, resumeTime)
// is not sent: it would not set the time back.
func restoreAccessTime(steps holdSteps, before uint64) string {
	if before == keepTime || before == holdTime || before == resumeTime {
		return reasonRestoreFailed
	}
	if steps.restore(before) != nil {
		return reasonRestoreFailed
	}
	if confirmed, err := steps.accessTime(); err != nil || confirmed != before {
		return reasonRestoreFailed
	}
	return reasonRestored
}
