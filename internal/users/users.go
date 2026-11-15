// Package users finds the home directories to collect. On Unix it reproduces
// the sh collector's enumerate_users, filter_users and get_user_homes; on
// Windows it reproduces the PowerShell collector's Get-CandidateHomes and
// Get-UserHomes, reading the registry's ProfileList in a live run.
package users

import (
	"context"
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

// Runner runs a program and returns its standard output, starting none once
// ctx is cancelled. hostcmd's Runner provides it, as a method value, so this
// package does not import hostcmd.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

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
// Profiles are empty off Windows. Found lists, once each in enumeration
// order, the users of the homes the run could collect from had -u named
// them: on Unix the candidates that are not system homes and are
// directories; on Windows those that are not skipped profiles or files,
// Skipped ones included. Unmatched lists the -u names, in the order typed,
// that match none of Found.
type Report struct {
	Homes     []Home
	Skipped   []Skipped
	Profiles  []Profile
	Found     []string
	Unmatched []string
}

// userNames splits the -u list at commas into its names in the order typed,
// without trimming, empty items dropped and a name that matches an earlier
// one under matches, the platform's comparison of names, dropped too.
func userNames(usersFilter string, matches func(name, user string) bool) []string {
	var names []string
	for _, name := range strings.Split(usersFilter, ",") {
		repeated := slices.ContainsFunc(names, func(earlier string) bool { return matches(earlier, name) })
		if name != "" && !repeated {
			names = append(names, name)
		}
	}
	return names
}

// unmatchedUsers returns the names, in order, that matches pairs with none
// of found.
func unmatchedUsers(names, found []string, matches func(name, user string) bool) []string {
	var unmatched []string
	for _, name := range names {
		if !slices.ContainsFunc(found, func(user string) bool { return matches(name, user) }) {
			unmatched = append(unmatched, name)
		}
	}
	return unmatched
}

// addFound appends user to found unless found holds it already.
func addFound(found []string, user string) []string {
	if slices.Contains(found, user) {
		return found
	}
	return append(found, user)
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
func Enumerate(ctx context.Context, live bool, root, usersFilter string, run Runner) []Home {
	return Find(ctx, live, root, usersFilter, run).Homes
}
