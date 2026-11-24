// Package fsmeta reads the file metadata the manifest records: on Unix the
// fields stat(1) gives and the unreadable-directory test; on Windows the
// fields, owner and attributes; and a directory tree's size everywhere.
//
// os.Lstat and fs.FileInfo carry a name, size, mode and modification time;
// the rest is behind Sys, in a type that differs by platform. Meta has the
// fields the manifest records on every platform: access, change and birth
// time (statx on Linux), owner and group, device, inode, link count, the
// Win32 attributes and the junction flag. On Windows the entry is opened
// without following a reparse point to read them. The package also opens files without following a link and holds the access
// time on Windows, which os.Open does neither.
package fsmeta
