package parserkit

import (
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/input"
)

// EntryKey names the entry a path opens, the key of a set of the files or
// directories a parser reads or shows once. macOS and Windows disks open
// one file by many spellings of its name (letter case, and on macOS how an
// accented letter is encoded), so EntryKey holds no form of the spelling:
// two paths that open one entry have one key on any disk. A device and
// inode number name a file only while it exists, so a set of keys lives
// in a PerHome, whose homes stay on disk while anything reads them.
type EntryKey struct {
	device, inode uint64
	// unnumbered is the path's own bytes when it names no entry with a
	// number, so two such paths are two keys and nothing is merged.
	unnumbered string
}

// KeyOf returns the key of the entry at path, its last component never
// followed: the device and inode number an lstat gives (the volume serial
// number and file index on Windows); when the path names nothing, cannot
// be looked at or has no such number, the path's own bytes.
func KeyOf(path string) EntryKey {
	meta, err := fsmeta.Lstat(path)
	if err != nil || meta.Ino == 0 {
		return EntryKey{unnumbered: path}
	}
	return EntryKey{device: meta.Dev, inode: meta.Ino}
}

// PerHome holds a value a parser keeps for each home it reads, such as a
// set of the entries it has read once, made on first use. A home is one
// opened collection's home, which stays on disk until every artifact of
// that collection has been parsed, so no entry a value names can be
// deleted and its number given to another file while the value is used.
// A parser value that outlives a collection never meets that home again,
// since a newly opened collection has new homes. An artifact with no home
// gets a new value each time.
type PerHome[T any] struct {
	values map[*input.Home]T
	fresh  func() T
}

// NewPerHome returns an empty PerHome whose values fresh makes.
func NewPerHome[T any](fresh func() T) *PerHome[T] {
	return &PerHome[T]{values: map[*input.Home]T{}, fresh: fresh}
}

// For returns the value of home, made on first use; a new value, not kept,
// for no home.
func (p *PerHome[T]) For(home *input.Home) T {
	if home == nil {
		return p.fresh()
	}
	value, ok := p.values[home]
	if !ok {
		value = p.fresh()
		p.values[home] = value
	}
	return value
}
