// Package parity holds the tests that run a reference collector and the
// doubleagent binary over the same image and compare what they write. They
// carry the parity build tag; the sh table is in the *_unix_test.go files.
// PARITY_REPO names the repository's root (default: three levels above this
// package) and PARITY_BINARY a prebuilt doubleagent (default: go build, which
// fails with a message when go is not on the PATH).
package parity
