package follow

import "github.com/seanthegeek/doubleagent/internal/fsmeta"

// CanRead reports whether path can be listed, by listing it: Windows has no permission bits to test, and a
// deny entry shows only when the listing fails.
func CanRead(check Check, opener *fsmeta.Opener, path string) (bool, error) {
	if err := LeavesRoot(check, path); err != nil {
		return false, err
	}
	_, failed, err := fsmeta.DirReadError(opener, path)
	return !failed, err
}

// CanEnter is always true: Windows lets every user pass through a directory
// it cannot list. A path out of the image root is still refused.
func CanEnter(check Check, path string) (bool, error) {
	if err := LeavesRoot(check, path); err != nil {
		return false, err
	}
	return true, nil
}
