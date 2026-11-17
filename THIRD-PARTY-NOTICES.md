# Third-party notices

DoubleAgent contains source code copied from other projects. Each copy keeps
its copyright line, and the licence text is stored beside it.

## zstd decompressor

- **Where:** [`internal/zstdframe`](internal/zstdframe), the files `bits.go`,
  `block.go`, `fse.go`, `huff.go`, `literals.go`, `window.go`, `xxhash.go` and
  `zstd.go`, with the tests and test data that go with them.
- **Origin:** the `internal/zstd` package of the Go standard library, version
  go1.27.4, copyright 2023 The Go Authors. The package name is the only change
  to the source apart from the test adjustments listed in
  [`internal/zstdframe/doc.go`](internal/zstdframe/doc.go).
- **Licence:** the Go licence (a three-clause BSD licence), reproduced in
  [`internal/zstdframe/LICENSE`](internal/zstdframe/LICENSE).
