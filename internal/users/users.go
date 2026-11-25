package users

import (
	"bytes"
	"context"
	"io"
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

// Skipped is a candidate home that could not be seen as a directory. User is
// the candidate's user (on a live Windows run the last segment of Path). Denied is true
// when its parent directory lists it, so it exists but cannot be entered;
// Reason says why in the platform's words.
type Skipped struct {
	User   string
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
// registry order before any filter (empty off Windows). Found lists once
// each, in enumeration order, the users whose home the run would collect had
// -u named them: candidates that are not system or shared profiles, files or
// (on Unix) missing, Skipped ones included. Unmatched lists the -u names, in
// the order typed, that match none of Found.
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

// inside returns the candidates whose own path the check allows, since the
// directory test and the walk after it go through a candidate that is a link.
func (guard *rootGuard) inside(candidates []Home) []Home {
	var kept []Home
	for _, home := range candidates {
		if guard.allows(home.Path) {
			kept = append(kept, home)
		}
	}
	return kept
}

// imageHomeGlobs returns the home parents relative to the root, in the order
// they are tried: an image run's on every platform, and a live Unix run's.
func imageHomeGlobs() []string {
	return []string{"home/*", "Users/*", "root", "var/root", "usr/home/*", "export/home/*"}
}

// homeRule is how a run judges its candidates: skip drops the home of a
// system or shared profile, and matches compares a -u name with a user.
type homeRule struct {
	skip    func(home Home) bool
	matches func(name, user string) bool
}

// imageRule returns an image run's rule, the same on every platform: a home
// is skipped by its path below root (skippedImageHome), and a -u name matches
// a user whatever the case of its letters (strings.EqualFold), since a name
// that matches too much only selects one more home.
func imageRule(root string) homeRule {
	return homeRule{
		skip:    func(home Home) bool { return skippedImageHome(strings.TrimPrefix(home.Path, root)) },
		matches: strings.EqualFold,
	}
}

// skippedImageHome reports whether a home, relative to the image root, is a
// system or shared profile's: a system path, or an entry directly under
// Users (that name's case ignored, ASCII only) whose name skipProfile
// matches. It goes by the path, not the user, so /home/temp is kept.
func skippedImageHome(relative string) bool {
	separator := strings.LastIndexByte(relative, '/')
	return isSystemPath(relative) || (separator >= 0 &&
		lowerASCII(relative[:separator]) == "/users" && skipProfile(relative[separator+1:]))
}

// isSystemPath reports whether a home, relative to the root, is a directory
// of the system that a service account's passwd line names, never a person's.
func isSystemPath(relative string) bool {
	switch relative {
	case "", "/", "/bin", "/sbin", "/usr", "/usr/bin", "/usr/sbin", "/dev", "/dev/null",
		"/proc", "/sys", "/nonexistent", "/var/empty":
		return true
	}
	return false
}

// passwdFileHomes returns the homes of root's etc/passwd (passwdHomes), read
// through readFile.
func passwdFileHomes(root string, guard *rootGuard) []Home {
	return passwdHomes(root, guard.readFile(root+"/etc/passwd"))
}

// passwdHomes returns the user and home of every line of text, a passwd
// file, with at least six fields and a non-empty sixth. The home is root and
// the field, respelled as the disk holds it below root (fsmeta.OnDiskPath),
// so it and a home parent's match of one directory are one candidate; a
// field this platform would read as another path (namesAnotherPath) gives
// none.
func passwdHomes(root string, text []byte) []Home {
	var homes []Home
	for _, line := range bytes.Split(text, []byte("\n")) {
		fields := strings.Split(string(line), ":")
		if len(fields) >= 6 && fields[5] != "" && !namesAnotherPath(fields[5]) {
			homes = append(homes, Home{fields[0], fsmeta.OnDiskPath(root, root+fields[5])})
		}
	}
	return homes
}

// readFile returns the bytes of path, its last component followed as cat
// follows it, once the root check lets it through (a refusal is recorded)
// and the opener admits the handle: on a Windows image, that it lies under
// the root. fsmeta.OpenDir is os.Open and that admission, which serve a file
// as they serve a directory. A directory or a missing file gives no text, a
// file that fails part way what was read before the failure.
func (guard *rootGuard) readFile(path string) []byte {
	if refusal, out := follow.OutOfRoot(follow.LeavesRoot(guard.check, path)); out {
		guard.record(refusal)
		return nil
	}
	file, err := fsmeta.OpenDir(guard.opener, path)
	if err != nil {
		return nil
	}
	defer file.Close()
	text, _ := io.ReadAll(file)
	return text
}

// named reports whether one of the -u names names user under matches. A
// non-empty list naming nobody (",") keeps no home.
func named(names []string, user string, matches func(name, user string) bool) bool {
	return slices.ContainsFunc(names, func(name string) bool { return matches(name, user) })
}

// userNames splits the -u list at commas into its names in the order typed,
// untrimmed, dropping empty items and a name that matches an earlier one
// under matches, the platform's comparison of names.
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
// profile, ignoring ASCII case only (U+017F does not match s).
func skipProfile(name string) bool {
	lowered := lowerASCII(name)
	fixed := [...]string{"public", "default", "default user", "all users", "shared", "defaultuser0"}
	return slices.Contains(fixed[:], lowered) || transientProfile.MatchString(lowered)
}

// lowerASCII lowers the ASCII letters of text and leaves every other byte.
func lowerASCII(text string) string {
	lowered := []byte(text)
	for i, character := range lowered {
		if 'A' <= character && character <= 'Z' {
			lowered[i] = character + 'a' - 'A'
		}
	}
	return string(lowered)
}
