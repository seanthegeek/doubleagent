//go:build unix

package collect

// logsAccessDecision is false: on Linux and macOS files are always opened
// for reading alone, so there is no decision to log.
const logsAccessDecision = false

// lastAccessUpdates is unknown: the mount options decide, per volume, and
// the copy opens read-only whatever they say.
func lastAccessUpdates() accessUpdates { return accessUnknown }
