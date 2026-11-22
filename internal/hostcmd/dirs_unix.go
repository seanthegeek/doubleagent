//go:build unix

package hostcmd

// systemDirectories are the directories searched for a program before the
// operator's PATH, in this order: the system's own programs, then
// /usr/local/bin.
func systemDirectories() []string {
	return []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/usr/local/bin"}
}
