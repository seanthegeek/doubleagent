package fsmeta

import (
	"io/fs"
	"os"
)

const (
	readBit    = 4
	executeBit = 1
)

// canAccess is true on Windows, which has no permission bits to test.
func canAccess(string, uint32) bool { return true }

// Lstat returns the metadata Windows gives through the standard library:
// size, modification time and kind. The other fields are 0.
func Lstat(path string) (Meta, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Meta{}, err
	}
	meta := Meta{Size: info.Size(), Mtime: info.ModTime().Unix(), Mode: "0"}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		meta.Kind = Symlink
	case info.IsDir():
		meta.Kind = Dir
	case info.Mode().IsRegular():
		meta.Kind = File
	}
	return meta, nil
}
