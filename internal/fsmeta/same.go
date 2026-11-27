package fsmeta

import (
	"errors"
	"io/fs"
)

// sameEntry is SameEntry with the lstat given.
func sameEntry(lstat func(string) (Meta, error), first, second string) bool {
	firstMeta, firstErr := lstat(first)
	secondMeta, secondErr := lstat(second)
	if errors.Is(firstErr, fs.ErrNotExist) && errors.Is(secondErr, fs.ErrNotExist) {
		return true
	}
	return firstErr == nil && secondErr == nil && firstMeta.Ino != 0 &&
		firstMeta.Dev == secondMeta.Dev && firstMeta.Ino == secondMeta.Ino
}

// SameEntry reports whether two spellings of a path, equal but for ASCII
// case, name one file system entry, never following the last component:
// yes when both lstats give one non-zero identity (volume serial number and
// file index on Windows, device and inode on Unix), and yes when neither
// exists, since two missing spellings hold no evidence a merge could lose.
// Any other answer keeps them apart: one missing, two identities, an
// identity of 0 (Lstat leaves it 0 for a file it cannot open), or an error
// other than not found, such as access denied.
func SameEntry(first, second string) bool {
	return sameEntry(Lstat, first, second)
}
