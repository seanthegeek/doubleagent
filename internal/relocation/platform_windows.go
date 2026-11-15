package relocation

// canEnter reports whether directory may be entered; Windows has no search
// permission bit, so every directory may.
func canEnter(string) bool { return true }
