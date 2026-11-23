// Package reason turns an error that stopped a read into the short text the
// analysis commands print in a `parser:` row, a `problem:` line or an error
// message.
//
// The common conditions of the operating system get one fixed lower-case
// reason each, the same on Linux, macOS and Windows: "permission denied",
// "no such file or directory", "is a directory", "not a directory", "too
// many levels of symbolic links" and "file name too long". Any other error
// gives the text of its innermost cause, without the operation and path Go
// puts in front of it.
//
// err.Error() names the operation and path and words a condition as each
// system does, so one denied file reads differently on each platform; this
// package gives one wording everywhere.
package reason
