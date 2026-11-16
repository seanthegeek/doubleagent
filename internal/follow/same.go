package follow

import (
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// SameEntry returns the run's answer to whether two spellings of a path
// whose syspath.Fold keys are equal name one file system entry: both pass
// the root check and their lstats give one non-zero identity (volume serial
// number and file index on Windows, device and inode on Unix). A path that
// cannot be examined, or whose identity is 0 (fsmeta.Lstat leaves it 0 when
// the file cannot be opened), proves nothing, so the two stay apart. The
// last component is never followed.
func SameEntry(check Check) syspath.SameEntry {
	return func(first, second string) bool {
		return sameIdentity(check, fsmeta.Lstat, first, second)
	}
}

// sameIdentity is SameEntry's answer with the lstat given.
func sameIdentity(check Check, lstat func(string) (fsmeta.Meta, error), first, second string,
) bool {
	if LeavesRoot(check, first) != nil || LeavesRoot(check, second) != nil {
		return false
	}
	firstMeta, err := lstat(first)
	if err != nil || firstMeta.Ino == 0 {
		return false
	}
	secondMeta, err := lstat(second)
	return err == nil && firstMeta.Dev == secondMeta.Dev && firstMeta.Ino == secondMeta.Ino
}
