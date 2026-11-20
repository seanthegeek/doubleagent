package parser

import "github.com/seanthegeek/doubleagent/internal/fsmeta"

// EntryKey names the file system entry a path opens, the key of a set or
// map of the files or directories a parser reads or shows once. A disk
// that matches names in any letter case (the default on macOS and
// Windows) opens one file for every case spelling of its name, so a key
// of the name as the evidence spells it would read that file once per
// spelling; EntryKey gives every spelling of one entry one key and two
// entries two keys, whatever the disk.
type EntryKey struct {
	device, inode uint64
	path          string
}

// KeyOf returns the key of the entry at path, its last component never
// followed: the device and inode number an lstat gives (the volume serial
// number and file index on Windows), else, when the path names nothing,
// cannot be looked at or has no such number, the path's own bytes.
func KeyOf(path string) EntryKey {
	meta, err := fsmeta.Lstat(path)
	if err != nil || meta.Ino == 0 {
		return EntryKey{path: path}
	}
	return EntryKey{device: meta.Dev, inode: meta.Ino}
}
