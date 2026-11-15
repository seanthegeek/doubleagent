// Package users finds the home directories to collect. On Unix it reproduces
// the sh collector's enumerate_users, filter_users and get_user_homes; on
// Windows it reproduces the PowerShell collector's Get-CandidateHomes and
// Get-UserHomes, reading the registry's ProfileList in a live run.
package users

import (
	"regexp"
	"slices"
	"strings"
)

// Home is a user and the home directory to collect for them. Path includes the
// image root, as the sh collector builds it.
type Home struct {
	User string
	Path string
}

// Runner runs a program and returns its standard output. hostcmd's Runner
// provides it, as a method value, so this package does not import hostcmd.
type Runner func(name string, args ...string) ([]byte, error)

// Skipped is a candidate home that could not be seen as a directory.
// Denied is true when its parent directory lists it, so it exists but cannot
// be entered; Reason says why in the platform's words.
type Skipped struct {
	Path   string
	Reason string
	Denied bool
}

// Profile is one entry of the registry's ProfileList: the key's security
// identifier, the user (the profile path's last component) and the profile
// path in canonical form without a trailing separator. It mirrors
// relocation.Profile; the collector converts between them.
type Profile struct {
	SecurityID string
	User       string
	Path       string
}

// Report is what Find returns: the homes to collect in enumeration order,
// the candidates that could not be seen as directories, and every
// ProfileList entry in registry order before any filter. Skipped and
// Profiles are empty off Windows.
type Report struct {
	Homes    []Home
	Skipped  []Skipped
	Profiles []Profile
}

// userNames splits the -u list at commas into its names in the order typed,
// without trimming, empty items and repeats dropped.
func userNames(usersFilter string) []string {
	var names []string
	for _, name := range strings.Split(usersFilter, ",") {
		if name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// transientProfile matches the ASCII-lowered names of the service profiles
// Windows makes for the font driver host, the desktop window manager and
// TEMP.
var transientProfile = regexp.MustCompile(`^(umfd-\d+|dwm-\d+|temp)(\..*)?$`)

// skipProfile reports whether a profile directory's name is a system or
// shared profile, not a person's home: one of the fixed names or a transient
// profile, compared ignoring ASCII case. PowerShell's -contains and -match
// also fold non-ASCII case pairs (U+017F for s); this does not.
func skipProfile(name string) bool {
	lowered := lowerASCII(name)
	fixed := [...]string{"public", "default", "default user", "all users", "shared", "defaultuser0"}
	for _, skipped := range fixed {
		if lowered == skipped {
			return true
		}
	}
	return transientProfile.MatchString(lowered)
}

// lowerASCII lowers the ASCII letters of text and leaves every other byte,
// on every platform.
func lowerASCII(text string) string {
	lowered := []byte(text)
	for i, character := range lowered {
		if 'A' <= character && character <= 'Z' {
			lowered[i] = character + 'a' - 'A'
		}
	}
	return string(lowered)
}

// Enumerate returns the homes to collect, in enumeration order; it is the
// homes of Find.
func Enumerate(live bool, root, usersFilter string, run Runner) []Home {
	return Find(live, root, usersFilter, run).Homes
}
