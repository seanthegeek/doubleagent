// Package parity holds the test that runs the sh collector and the
// doubleagent binary over the smoke test's image and compares what they
// write. The test files carry the parity build tag: they run only under
// `go test -tags parity`, because they need dash, the system tools and many
// minutes.
package parity
