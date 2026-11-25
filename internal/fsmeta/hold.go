package fsmeta

// This file is compiled on every platform, though only the Windows opener
// calls it, so every CI runner tests the decision.

// The reasons Opener.Report receives for a file read without the hold on
// its access time that the run wanted.
const (
	reasonRightRefused = "right to hold refused"
	reasonHoldRefused  = "hold refused"
	reasonNotVerified  = "hold not verified"
	reasonChanged      = "access time changed while the hold was asked for"
)

// holdSteps are the operations verifyHold runs on one open file: read its
// access time, and ask that reads not update it.
type holdSteps struct {
	accessTime func() (uint64, error)
	hold       func() error
}

// verifyHold asks for the hold, reading the access time before and after,
// and returns the reason to report ("" when the hold is in place). It only
// reports: a time that changed is left as it is, since the run never sets
// a source's access time.
func verifyHold(steps holdSteps) string {
	before, beforeErr := steps.accessTime()
	holdErr := steps.hold()
	after, afterErr := steps.accessTime()
	switch {
	case holdErr != nil:
		return reasonHoldRefused
	case beforeErr != nil || afterErr != nil:
		return reasonNotVerified
	case after != before:
		return reasonChanged
	}
	return ""
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
