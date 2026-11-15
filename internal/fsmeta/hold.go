package fsmeta

// This file is compiled on every platform, though only the Windows opener
// calls it, so every CI runner tests the decision.

// The reasons Opener.Report receives for a file read without the hold on
// its access time that the run wanted.
const (
	reasonRightRefused  = "right to hold refused"
	reasonHoldRefused   = "hold refused"
	reasonNotVerified   = "hold not verified"
	reasonChanged       = "access time changed while the hold was asked for"
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

// Allowances of the misread test, in 100-nanosecond intervals. A time up to
// a day ahead of this machine's clock may be genuine: a file server stamps
// accesses with its own clock, which may run hours ahead. A time up to two
// seconds behind the value read before may be the same time read at a
// coarser grain (FAT keeps write times to two seconds).
const (
	futureAllowance = 24 * 60 * 60 * 10_000_000
	grainAllowance  = 2 * 10_000_000
)

// holdSteps are the operations verifyHold runs on one open file: read its
// access time, ask that reads not update it, set it to a given value (and
// no other time), and read this machine's clock in the same units.
type holdSteps struct {
	accessTime func() (uint64, error)
	hold       func() error
	restore    func(accessTime uint64) error
	now        func() uint64
}

// verifyHold asks for the hold, reading the access time before and after,
// and returns the reason to report ("" when the hold is in place). Only a
// value no access could have written (misread) is taken as a misread hold
// and set back; another reader may move the time forward meanwhile, and
// that record is kept. The answer to the hold is judged after the reads,
// since a driver may store the value and still return an error.
func verifyHold(steps holdSteps) string {
	before, beforeErr := steps.accessTime()
	holdErr := steps.hold()
	after, afterErr := steps.accessTime()
	switch {
	case afterErr == nil && beforeErr == nil && misread(before, after, steps.now()):
		return restoreAccessTime(steps, before)
	case afterErr == nil && misread(after, after, steps.now()):
		// The first read failed: the misread is certain, the old time unknown.
		return reasonRestoreFailed
	case holdErr != nil:
		return reasonHoldRefused
	case beforeErr != nil || afterErr != nil:
		return reasonNotVerified
	case after > before:
		return reasonChanged
	}
	// The same time, or one behind by less than the grain allowance.
	return ""
}

// misread reports whether after, read once the hold was asked for, is a
// value no access could have written: the hold value itself, a time more
// than futureAllowance ahead of now, or a time earlier than before by more
// than grainAllowance (an access never moves the time back), as a server
// taking all ones as the signed number -1 would store, just before 1601.
func misread(before, after, now uint64) bool {
	return after == holdTime || after > now+futureAllowance ||
		(after < before && before-after > grainAllowance)
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

// reportsOnVolume reports whether a file read without the hold for reason
// is reported. A refusal (reasonRightRefused, reasonHoldRefused) on a
// volume that is itself read-only (readOnlyVolume, asked only then) is
// not: no read there can move an access time.
func reportsOnVolume(reason string, readOnlyVolume func() bool) bool {
	if reason != reasonRightRefused && reason != reasonHoldRefused {
		return reason != ""
	}
	return !readOnlyVolume()
}
