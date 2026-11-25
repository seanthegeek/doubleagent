package users

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// homeGlobs returns the home parents relative to the base, in the order they
// are tried: a live run looks under the system drive's Users and home, an
// image under imageHomeGlobs.
func homeGlobs(live bool) []string {
	if live {
		return []string{"Users/*", "home/*"}
	}
	return imageHomeGlobs()
}

// namesAnotherPath reports whether a passwd home field would name a path
// other than the image's own: one holding a backslash, which Windows reads
// as a separator, so the open would reach another directory.
func namesAnotherPath(field string) bool { return strings.ContainsRune(field, '\\') }

// errorInvalidName is ERROR_INVALID_NAME, which Windows gives for a path
// holding a character no Windows name may hold: a passwd home so named
// cannot exist there.
const errorInvalidName = syscall.Errno(123)

// liveRule is a live run's rule: a profile is skipped by its directory's
// name (skipProfile), and a -u name matches a user ignoring every letter's
// case (strings.EqualFold). Path keys fold ASCII only because merging two
// names NTFS keeps apart would lose evidence; a -u name that matches too
// much only selects one more home.
func liveRule() homeRule {
	return homeRule{skip: func(home Home) bool { return skipProfile(home.User) },
		matches: strings.EqualFold}
}

// Find returns the homes to collect, in enumeration order. A live run reads
// the registry's ProfileList, then adds the directories under the system
// drive's home parents (liveRule); an image run reads root's etc/passwd,
// then adds the directories under imageHomeGlobs (imageRule). A candidate
// that is a file is dropped; one whose entry cannot be looked at is in
// Skipped, but an image's that does not exist is dropped. Both tests read the entry's own attributes, a link not followed.
// ctx and run are unused (no program lists Windows users); check is the
// image root's, nil live.
func Find(_ context.Context, live bool, root, usersFilter string, _ Runner,
	check glob.Check, opener *fsmeta.Opener, reach func(path string) string,
) Report {
	var reader profileReader
	if live {
		reader = registryProfiles{}
	}
	return find(reader, root, usersFilter, check, opener, reach)
}

// find is Find with the ProfileList reader given; a nil reader means an
// image run.
func find(reader profileReader, root, usersFilter string, check glob.Check,
	opener *fsmeta.Opener, reach func(path string) string,
) Report {
	root = syspath.FromNative(root)
	guard := &rootGuard{check: check, opener: opener}
	var report Report
	var candidates []Home
	rule := imageRule(root)
	if reader != nil {
		rule = liveRule()
		// A ProfileList path is the registry's spelling, which may differ
		// in case from the directory the Users listing gives; respelled as
		// the disk holds it, one directory is one candidate path.
		for _, profile := range reader.Profiles() {
			profile.Path = fsmeta.OnDiskPath("", profile.Path)
			report.Profiles = append(report.Profiles, profile)
			candidates = append(candidates, Home{profile.User, profile.Path})
		}
	} else {
		candidates = passwdFileHomes(root, guard)
	}
	candidates = append(candidates, globHomes(syspath.SystemBase(root), reader != nil, guard)...)
	candidates = guard.inside(candidates)
	candidates, written := reachAll(candidates, reach)
	same := follow.SameEntry(check)
	candidates, written = dropReachedRoots(candidates, written, root, same)
	report.Homes, report.Skipped, report.Found, report.Merged = filterProfiles(candidates,
		written, usersFilter, same, rule, reader != nil)
	report.OutOfRoot = guard.links
	report.Unmatched = unmatchedUsers(userNames(usersFilter, rule.matches), report.Found,
		rule.matches)
	return report
}

// globHomes returns the directories under each home parent of base, each
// named by its last path segment.
func globHomes(base string, live bool, guard *rootGuard) []Home {
	var homes []Home
	for _, pattern := range homeGlobs(live) {
		for _, match := range guard.expand(base, pattern) {
			homes = append(homes, Home{match[strings.LastIndexByte(match, '/')+1:], match})
		}
	}
	return homes
}

// filterProfiles applies rule's skip, the file test, the -u list under
// rule's comparison and the directory test to the candidates in order, and
// drops repeats. One look at each entry's own attributes serves both tests:
// without the directory attribute it is a file, whatever a file symlink
// leads to; with it, it is a home, so a junction or directory symlink whose
// target is gone is collected, with its error_read row. Two paths are one
// home only when same says so. A skipped candidate is named by its user, on
// a live run by its path's last segment; an image's candidate that does not
// exist or that Windows cannot name (a passwd home) is dropped, as on Unix,
// while a live run keeps a ProfileList entry whose directory is gone.
func filterProfiles(candidates []Home, written []string, usersFilter string,
	same syspath.SameEntry, rule homeRule, live bool,
) ([]Home, []Skipped, []string, []Merge) {
	names := userNames(usersFilter, rule.matches)
	var homes []Home
	var skipped []Skipped
	var found []string
	var merged []Merge
	seenHomes, seenSkipped := syspath.NewEntries[int](same), syspath.NewEntries[bool](same)
	for i, candidate := range candidates {
		if rule.skip(candidate) {
			continue
		}
		attributes, err := entryAttributes(candidate.Path)
		if err == nil && attributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 {
			continue
		}
		if !live && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, errorInvalidName)) {
			continue
		}
		found = addFound(found, candidate.User)
		if usersFilter != "" && !named(names, candidate.User, rule.matches) {
			continue
		}
		if err == nil {
			first, _, seen := seenHomes.Find(candidate.Path)
			switch {
			case !seen:
				homes = append(homes, candidate)
				seenHomes.Put(candidate.Path, i)
			case !syspath.Equal(written[first], written[i]):
				merged = append(merged, Merge{Path: candidate.Path,
					Kept: candidates[first].User, Dropped: candidate.User})
			}
		} else if seenSkipped.Add(candidate.Path, true) {
			user := candidate.User
			if live {
				user = leaf(candidate.Path)
			}
			skipped = append(skipped, skippedHome(user, candidate.Path, err))
		}
	}
	return homes, skipped, found, merged
}

// dropReachedRoots drops each candidate that reaching turned into the image
// root itself, or on a live run (root "") into a drive root, which is never
// a home; the run's skip rule still applies to the others. A path spelled
// otherwise than root is root only when same says so.
func dropReachedRoots(candidates []Home, written []string, root string, same syspath.SameEntry,
) ([]Home, []string) {
	var keptHomes []Home
	var keptWritten []string
	for i, candidate := range candidates {
		path := candidate.Path
		isRoot := (root != "" && syspath.SameAs(path, root, same)) ||
			(root == "" && len(path) == 2 && path[1] == ':')
		if path != written[i] && isRoot {
			continue
		}
		keptHomes, keptWritten = append(keptHomes, candidate), append(keptWritten, written[i])
	}
	return keptHomes, keptWritten
}

// entryAttributes returns the file attributes of path itself, a link not
// followed, or the error of looking at it.
func entryAttributes(path string) (uint32, error) {
	info, err := osfile.Lstat(path)
	if err != nil {
		return 0, err
	}
	// os.Lstat on Windows always describes an entry with this type.
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return 0, errors.New("lstat " + syspath.ToNative(path) + ": no file attributes")
	}
	return data.FileAttributes, nil
}

// skippedHome describes user's candidate home at path, whose entry could not
// be looked at, with the text of err, Denied when its parent lists it. On
// NTFS whoever may list a directory may read its entries' attributes, so the
// usual case is a candidate that is gone or whose parent cannot be listed;
// Denied covers an entry that changed between the two looks or lies on
// another file system (a network share decides for itself), and the
// error_read row of such a home.
func skippedHome(user, path string, err error) Skipped {
	return Skipped{User: user, Path: path,
		Reason: err.Error(), Denied: parentLists(path)}
}
