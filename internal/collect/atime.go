package collect

// accessUpdates says whether the system updates a file's last-access time
// when the file is read, as far as the run could tell.
type accessUpdates int

// The states of last-access updates. accessUnknown is a setting that is
// missing, unreadable or of a form the run cannot interpret.
const (
	accessUnknown accessUpdates = iota
	accessOn
	accessOff
)

// accessUpdatesOf interprets the NtfsDisableLastAccessUpdate registry value.
// Microsoft documents 1 (updates off) and 0 (updates on); recent Windows
// versions write the same bit with 0x80000000 added, and 2 added when the
// system manages the setting, so the four values 0x80000000 to 0x80000003
// are read by their lowest bit as well. A test on Windows compares this
// reading with what a read of a file measures. Any other value, or a value
// that is not a 32-bit number, is unknown.
func accessUpdatesOf(value uint64, isDWORD bool) accessUpdates {
	if !isDWORD {
		return accessUnknown
	}
	switch value {
	case 0, 0x80000000, 0x80000002:
		return accessOn
	case 1, 0x80000001, 0x80000003:
		return accessOff
	}
	return accessUnknown
}

// accessDecisionLine is the log line that says how collected files are
// opened and why.
func accessDecisionLine(state accessUpdates, noPreserveAtime bool) string {
	switch {
	case noPreserveAtime:
		return "--no-preserve-atime: files are opened read-only; reading may change their access time on disk"
	case state == accessOff:
		return "last-access updates are off for NTFS on this system; files on NTFS are opened read-only, " +
			"others with the right to hold their access time"
	case state == accessOn:
		return "last-access updates are on; files are opened with the right to hold their access time"
	}
	return "last-access updates could not be determined; files are opened with the right to hold their access time"
}
