package fsmeta

import "strings"

// notDirectoryError is the error for a path that is not a directory: the
// type error when something is there (a file, or a dangling link), else the
// error Lstat gives for the missing path.
func notDirectoryError(path string) error {
	_, err := Lstat(path)
	if err == nil {
		return pathError("open", path, errNotDirectory)
	}
	return err
}

// firstLine returns text up to its first newline.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}
