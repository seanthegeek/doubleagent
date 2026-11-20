package parser

import "github.com/seanthegeek/doubleagent/internal/fsmeta"

// EntryKey names the file system entry a path opens, the key of a set or
// map of the files or directories a parser reads or shows once. A disk
// that matches names in any letter case (the default on macOS and
// Windows) opens one file for every case spelling of its name, so a key
// of the name as the evidence spells it would read that file once per
// spelling; EntryKey gives the case spellings of one entry one key and two
// entries two keys, whatever the disk.
type EntryKey struct {
	device, inode uint64
	// folded is the path with its ASCII letters in lower case. The device
	// and inode number name a file only while it exists, and a deleted
	// file's number is soon given to a new file elsewhere; with the path's
	// fold beside them, two keys are equal only for two case spellings of
	// one path that open one entry.
	folded string
}

// KeyOf returns the key of the entry at path, its last component never
// followed: its path folded to lower case in ASCII, with the device and
// inode number an lstat gives (the volume serial number and file index on
// Windows); when the path names nothing, cannot be looked at or has no
// such number, the path's own bytes alone.
func KeyOf(path string) EntryKey {
	meta, err := fsmeta.Lstat(path)
	if err != nil || meta.Ino == 0 {
		return EntryKey{folded: path}
	}
	return EntryKey{device: meta.Dev, inode: meta.Ino, folded: foldASCII(path)}
}

// foldASCII returns text with each ASCII capital letter in lower case and
// every other byte as it is, so no two byte strings fold alike unless they
// differ only in ASCII case.
func foldASCII(text string) string {
	folded := []byte(text)
	for i, character := range folded {
		if 'A' <= character && character <= 'Z' {
			folded[i] = character + 'a' - 'A'
		}
	}
	return string(folded)
}
