// Package zstdframe decodes zstd data for the analyzer: a whole buffer of
// frames, or a stream where a cut-off input is told apart from a corrupt one.
//
// The files bits.go, block.go, fse.go, huff.go, literals.go, window.go,
// xxhash.go and zstd.go are a copy of the Go standard library's internal/zstd
// package (Go 1.27.4), a decompressor for the format of RFC 8878, with only
// the package name changed. They are used under the Go licence, whose text is
// in the file LICENSE beside them; each file keeps its copyright line. The
// copy is here because that package is internal to the standard library and
// cannot be imported.
//
// The tests and test data of that package are copied too. Two imports that
// exist only inside the Go tree were replaced (internal/race by race_on_test.go
// and race_off_test.go, internal/testenv by nothing), and the path of the
// shared text file they read now points into testdata. The tests that compare
// against the zstd and xxhsum programs skip when those are not installed.
//
// The file decode.go is this project's own.
//
// The decoder does not support dictionaries. It keeps at most MaxWindowSize
// bytes of earlier output for back references, whatever window a frame
// announces; a frame whose matches reach further back than that fails with a
// corrupt-data error.
package zstdframe
