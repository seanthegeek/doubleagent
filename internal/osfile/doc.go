// Package osfile wraps the os functions that take a path, so that on Windows
// every file operation names exactly the entry it was given.
//
// Windows normalises an ordinary path such as C:\dir\name before using it: a
// dot or a space ending a path component is dropped. NTFS itself stores such
// names, and other software can create them. Opened through an ordinary path,
// a directory named "x." is therefore not found, or, when a neighbour named
// "x" exists, the neighbour is opened in its place without an error. A
// run that listed "x." and then read "x" would record one entry's name
// and metadata beside another entry's contents and hash.
//
// A path with the \\?\ prefix is passed to the file system as written, with no
// normalisation, so it reaches the entry it names; the same form lifts the
// 260-character path limit. The os package adds that prefix only to a long
// path. Every function here calls its os counterpart on syspath.Extended(path),
// which adds the prefix to any absolute path, so listing, describing, reading
// and writing all agree on which entry a path means. Code built for Windows
// calls these functions and never the os ones; a test in this package fails
// on a direct call.
//
// Errors come back with their paths in native form, without the prefix. A
// returned *os.File keeps the extended form as its Name, and errors from its
// methods are not rewritten: a caller that reports one passes it through
// syspath.NativeErrors. On Unix a path is used as written, so each function
// is its os counterpart.
package osfile
