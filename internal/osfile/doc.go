// Package osfile wraps the os functions that take a path, so that on Windows
// every file operation names exactly the entry it was given.
//
// Windows normalises an ordinary path such as C:\dir\name before using it,
// dropping a dot or a space that ends a component. NTFS stores such names,
// and other software creates them, so through an ordinary path a directory
// named "x." is not found, or, when a neighbour named "x" exists, the
// neighbour is opened in its place without an error: a row would pair one
// entry's name and metadata with another entry's contents and hash.
//
// A path with the \\?\ prefix reaches the entry it names, unnormalised and
// past the 260-character limit. The os package adds that prefix only to a
// long path; every function here calls its os counterpart on
// syspath.Extended(path), which adds it to any absolute path. Code built
// for Windows calls these functions, never the os ones; a test in this
// package fails on a direct call.
//
// Errors come back with their paths in native form. A returned *os.File
// keeps the extended form as its Name, and its methods' errors are not
// rewritten: a caller that reports one passes it through
// syspath.NativeErrors. On Unix each function is its os counterpart.
//
// Spellings adds the one lookup os has no counterpart for: a name a reader
// built, found in a directory's listing whatever the ASCII case of its
// letters, for a collection read on a disk that keeps case apart.
package osfile
