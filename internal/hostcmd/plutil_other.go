//go:build !darwin

package hostcmd

// plutilPlatform is false everywhere but macOS, where no plutil of Apple's
// exists and a program of that name does not answer as it does.
const plutilPlatform = false
