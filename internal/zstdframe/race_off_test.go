//go:build !race

package zstdframe

// raceEnabled stands in for the Go tree's internal/race, which a package
// outside the standard library cannot import.
const raceEnabled = false
