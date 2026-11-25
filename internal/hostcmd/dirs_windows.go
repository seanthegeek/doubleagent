package hostcmd

// systemDirectories is empty on Windows, which has none of the Unix system
// directories: a name is looked up along PATH alone, each candidate tried
// with the PATHEXT extensions as exec.LookPath tries them.
func systemDirectories() []string {
	return nil
}
