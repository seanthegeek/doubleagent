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

// The bounds of a possible access time, in FILETIME units. floorTime is the
// first day of 1980, the earliest date FAT can hold: no file system Windows
// reads stamps an access before it, and a server that takes all ones as the
// signed number -1 stores a date at or just before 1601, or 1970 if it
// clamps. ceilingSpan is 50 years past this machine's clock, more than any
// clock error: all ones read as an unsigned time is tens of thousands of
// years ahead. Of the dates servers clamp to, 2107 (for about thirty more
// years), 2262 and 9999 lie past the ceiling; the end of 32-bit Unix time
// does not, and a server clamping there is not caught.
const (
	floorTime   = 119_600_064_000_000_000
	ceilingSpan = 50 * 36525 * 24 * 60 * 60 * 10_000_000 / 100 // 365.25 days a year
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
// time that changed to a value no real access could have written (misread)
// is taken as a misread hold and set back. Any other change is reported
// and left as it is: another program's read through its own open, stamped
// by a clock that may be far from this machine's. The answer to the hold is
// judged after the reads, since a driver may store the value and still
// return an error.
func verifyHold(steps holdSteps) string {
	before, beforeErr := steps.accessTime()
	holdErr := steps.hold()
	after, afterErr := steps.accessTime()
	switch {
	case afterErr == nil && beforeErr == nil && misread(before, after, steps.now()):
		return restoreAccessTime(steps, before)
	case afterErr == nil && beforeErr != nil && after == holdTime:
		// Only the hold value itself is a certain misread with no first
		// reading; the old time is unknown, so nothing can be set back.
		return reasonRestoreFailed
	case holdErr != nil:
		return reasonHoldRefused
	case beforeErr != nil || afterErr != nil:
		return reasonNotVerified
	case after != before:
		return reasonChanged
	}
	return ""
}

// misread reports whether after, read once the hold was asked for, differs
// from before and cannot be the time of any real access: the hold value
// itself, a time before floorTime, or one more than ceilingSpan past now.
func misread(before, after, now uint64) bool {
	return after != before &&
		(after == holdTime || after < floorTime || after > now+ceilingSpan)
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
