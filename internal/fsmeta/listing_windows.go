package fsmeta

import (
	"io/fs"
	"syscall"
)

// ListingOrder returns entries in the order PowerShell's Get-ChildItem gives
// a directory's children: first every entry Windows marks a directory (a
// junction or a directory symlink included), then the rest, each group in
// the order the listing returned it. Its FileSystem provider enumerates a
// directory's subdirectories and its files in two passes, in that order.
func ListingOrder(entries []fs.DirEntry) []fs.DirEntry {
	ordered := make([]fs.DirEntry, 0, len(entries))
	for _, wantDirectory := range []bool{true, false} {
		for _, entry := range entries {
			if markedDirectory(entry) == wantDirectory {
				ordered = append(ordered, entry)
			}
		}
	}
	return ordered
}

// markedDirectory reports whether the listing gave entry the directory
// attribute, the test .NET's directory enumeration applies; Go types a
// junction as irregular, so the entry's type cannot answer it. The os
// package hands the attributes over as the syscall package's type.
func markedDirectory(entry fs.DirEntry) bool {
	info, err := entry.Info()
	if err != nil {
		return entry.IsDir()
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return entry.IsDir()
	}
	return data.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0
}
