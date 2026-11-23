// Package relocation finds the startup files that name an environment
// variable able to move an agent's files (a relocation hint), reads the value
// a line assigns where the file's syntax needs no shell, and records each hint
// with its log lines and collection.json entries.
// docs/relocation.md and the "Relocation hints" section of
// docs/internals.md describe the step. Collecting a hit file and
// listing the catalog's matches are the caller's, through Deps.
package relocation
