// Package users finds the home directories to collect: on Unix from the
// password database, dscl and the usual home parents; on Windows from the
// home parents, reading the registry's ProfileList in a live run.
package users
