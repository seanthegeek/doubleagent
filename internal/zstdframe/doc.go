// Package zstdframe decodes zstd data for the analyzer: a whole buffer of
// frames, or a stream where a cut-off input is told apart from a corrupt one.
//
// The decoding itself is done by the zstd package of the module
// github.com/klauspost/compress (vendored, see the dependency table in
// AGENTS.md). This package configures that decoder to run in the calling
// goroutine, to refuse a frame whose window is above MaxWindowSize, and to
// refuse output above the caller's limit, and it sorts the decoder's failures
// into ErrTruncated, ErrCorrupt and ErrTooLarge.
//
// Dictionaries are not supported: a frame that names one is reported as
// corrupt.
package zstdframe
