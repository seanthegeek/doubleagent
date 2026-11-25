package users

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Home is a user and the home directory to collect for them. Path includes the
// image root.
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
// path in canonical form without a trailing separator.
type Profile struct {
	SecurityID string
	User       string
	Path       string
}

// Report is what Find returns: the homes to collect in enumeration order,
// the candidates that could not be examined, and every ProfileList entry in
// registry order before any filter. Profiles is empty off Windows. Found
// lists, once each in enumeration order, the users of the candidates that
// are not system or shared profiles, files or (on Unix) missing, Skipped
// ones included: those whose homes the run would collect had -u named them. Unmatched lists the -u names, in the order typed,
// that match none of Found.
type Report struct {
	Homes     []Home
	Skipped   []Skipped
	Profiles  []Profile
	Found     []string
	Unmatched []string
	// OutOfRoot lists, once each, the links out of the image root that the
	// enumeration did not look behind (a home parent, the passwd file's
	// directory, a candidate home); none of them gives a home.
	OutOfRoot []follow.Refusal
	// Merged lists the homes dropped because, once reached, they name the
	// directory an earlier home names: walked once, as the earlier user's.
	Merged []Merge
}

// Merge is a home Find dropped because its path, once reached, is the path
// of an earlier home: Path, walked once as Kept's, is also Dropped's.
type Merge struct {
	Path    string
	Kept    string
	Dropped string
}

// reachAll returns candidates with each path put through reach (nil:
// unchanged), and the paths as written, in the same order.
func reachAll(candidates []Home, reach func(path string) string) (reached []Home,
	written []string,
) {
	for _, candidate := range candidates {
		written = append(written, candidate.Path)
		if reach != nil && candidate.Path != "" {
			candidate.Path = reach(candidate.Path)
		}
		reached = append(reached, candidate)
	}
	return reached, written
}

// rootGuard applies an image run's root check to what the enumeration lists
// and stats; nil check, a live run's, allows everything. Each refused link is
// recorded once.
type rootGuard struct {
	check  glob.Check
	opener *fsmeta.Opener
	links  []follow.Refusal
}

// allows reports whether path may be looked at, recording the link when not.
func (guard *rootGuard) allows(path string) bool {
	if guard.check == nil {
		return true
	}
	refusal, refused := guard.check(path)
	if refused {
		guard.record(refusal)
	}
	return !refused
}

// record keeps refusal once per link.
func (guard *rootGuard) record(refusal follow.Refusal) {
	if !slices.ContainsFunc(guard.links, func(known follow.Refusal) bool {
		return known.Link == refusal.Link
	}) {
		guard.links = append(guard.links, refusal)
	}
}

// expand expands pattern in base through the check, recording what it refuses.
func (guard *rootGuard) expand(base, pattern string) []string {
	matches, refused := glob.Expand(base, pattern, guard.check, guard.opener)
	for _, refusal := range refused {
		guard.record(refusal.Refusal)
	}
	return matches
}

// inside returns the candidates whose own path the check allows: test -d,
// and the walk after it, would follow a candidate that is itself a link.
func (guard *rootGuard) inside(candidates []Home) []Home {
	var kept []Home
	for _, home := range candidates {
		if guard.allows(home.Path) {
			kept = append(kept, home)
		}
	}
	return kept
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

// parentLists reports whether the directory holding path lists an entry of
// that name, compared as syspath.Equal compares paths.
func parentLists(path string) bool {
	separator := strings.LastIndexByte(path, '/')
	if separator < 0 {
		return false
	}
	parent, leaf := path[:separator], path[separator+1:]
	if len(parent) == 2 && parent[1] == ':' {
		parent += "/" // a bare Windows drive names the drive's current directory
	}
	entries, err := osfile.ReadDir(parent)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if syspath.Equal(entry.Name(), leaf) {
			return true
		}
	}
	return false
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
// profile, compared ignoring ASCII case only: a non-ASCII case pair (U+017F
// for s) is not folded.
func skipProfile(name string) bool {
	lowered := lowerASCII(name)
	fixed := [...]string{"public", "default", "default user", "all users", "shared", "defaultuser0"}
	return slices.Contains(fixed[:], lowered) || transientProfile.MatchString(lowered)
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
func Enumerate(ctx context.Context, live bool, root, usersFilter string, run Runner,
	check glob.Check, opener *fsmeta.Opener, reach func(path string) string,
) []Home {
	return Find(ctx, live, root, usersFilter, run, check, opener, reach).Homes
}
