//go:build !race

package growthtests

// raceDetector reports whether the test binary was built with the race
// detector, which changes what a parse allocates and slows it.
const raceDetector = false
