// Package fsmeta reads the file metadata the manifest records, tests
// whether a directory can be listed and entered, and sizes a directory tree.
//
// os.Lstat and fs.FileInfo carry a name, size, mode and modification time;
// the rest is behind Sys, in a type that differs by platform. Meta has the
// fields the manifest records on every platform: access, change and birth
// time (statx on Linux), owner and group, device, inode, link count, the
// Win32 attributes and the junction flag; on Windows they are read from a
// handle opened without following a reparse point. The package also opens
// files without following a link and, on Windows, holds their access time,
// neither of which os.Open does.
package fsmeta
