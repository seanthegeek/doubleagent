package relocation

// noFollow is 0: Windows has no O_NOFOLLOW, and the caller has already found
// the path to be a regular file with Lstat.
const noFollow = 0

// canEnter reports whether directory may be entered; Windows has no search
// permission bit, so every directory may.
func canEnter(string) bool { return true }
