// Package zstdframe decodes zstd data for the analysis commands as a
// stream, where a cut-off input is told apart from a corrupt one.
//
// The decoding itself is done by the zstd package of the module
// github.com/klauspost/compress (vendored, see the dependency table in
// internal/AGENTS.md). This package configures that decoder to run in the
// calling goroutine and to refuse a frame whose window is above
// MaxWindowSize or the caller's own lower limit, and it sorts the
// decoder's failures into ErrTruncated, ErrCorrupt and ErrTooLarge. A
// Decoder keeps its buffers from one input to the next, for a caller that
// decodes many small inputs. Bytes after the last frame are read as the
// start of another: fewer than the four of a magic number are truncation,
// four or more that are none are corruption.
//
// Dictionaries are not supported: a frame that names one is reported as
// corrupt.
//
// The standard library's own decoder, internal/zstd, cannot be imported;
// using the vendored decoder directly would leave each caller to set the
// concurrency and window limits and to sort its errors, which this
// package does once.
package zstdframe
