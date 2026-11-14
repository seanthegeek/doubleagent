package manifest

// appendPlatformFields adds the owner and attributes fields the PowerShell
// collector writes after mode.
func appendPlatformFields(out []byte, row Row) []byte {
	return appendWindowsFields(out, row)
}
