package users

import (
	"context"
	"errors"
	"slices"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// homeGlobs returns the home parents relative to the base, in the order they
// are tried: a live run looks under the system drive, an image also at the
// Unix home parents, Users first in both.
func homeGlobs(live bool) []string {
	if live {
		return []string{"Users/*", "home/*"}
	}
	return []string{"Users/*", "home/*", "root", "var/root", "usr/home/*", "export/home/*"}
}

// Find returns the homes to collect, in enumeration order. A live run reads
// the registry's ProfileList first, then both modes add the directories
// under the home parents of root (the system drive when live). A candidate
// that is a file is dropped; one whose entry cannot be looked at is in
// Skipped. Both tests read the entry's own attributes, a link not followed.
// The -u list ignores every letter's case (wantedUser says why). ctx and run
// are unused (no program lists Windows users); check is the image root's,
// nil live.
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
	if reader != nil {
		// A ProfileList path is the registry's spelling, which may differ
		// in case from the directory the Users listing gives; respelled as
		// the disk holds it, one directory is one candidate path.
		for _, profile := range reader.Profiles() {
			profile.Path = fsmeta.OnDiskPath("", profile.Path)
			report.Profiles = append(report.Profiles, profile)
			candidates = append(candidates, Home{profile.User, profile.Path})
		}
	}
	candidates = append(candidates, globHomes(syspath.SystemBase(root), reader != nil, guard)...)
	candidates = guard.inside(candidates)
	candidates, written := reachAll(candidates, reach)
	same := follow.SameEntry(check)
	candidates, written = dropReachedRoots(candidates, written, root, same)
	report.Homes, report.Skipped, report.Found, report.Merged = filterProfiles(candidates,
		written, usersFilter, same)
	report.OutOfRoot = guard.links
	report.Unmatched = unmatchedUsers(userNames(usersFilter, strings.EqualFold), report.Found,
		strings.EqualFold)
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

// filterProfiles applies the skip list, the file test, the -u list and the
// directory test to the candidates in order, and drops repeats. One look at
// each entry's own attributes serves both tests: without the directory
// attribute it is a file, so a file symlink is a file whatever it leads to;
// with it, it is a home, so a junction or directory symlink whose target is
// gone is collected, with its error_read row, rather than skipped. Two paths
// are one home only when same says so: in a case-sensitive Users directory,
// Alice and alice are two homes.
func filterProfiles(candidates []Home, written []string, usersFilter string,
	same syspath.SameEntry,
) ([]Home, []Skipped, []string, []Merge) {
	names := userNames(usersFilter, strings.EqualFold)
	var homes []Home
	var skipped []Skipped
	var found []string
	var merged []Merge
	seenHomes, seenSkipped := syspath.NewEntries[int](same), syspath.NewEntries[bool](same)
	for i, candidate := range candidates {
		if skipProfile(candidate.User) {
			continue
		}
		attributes, err := entryAttributes(candidate.Path)
		if err == nil && attributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 {
			continue
		}
		found = addFound(found, candidate.User)
		if usersFilter != "" && !wantedUser(names, candidate.User) {
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
		} else {
			if seenSkipped.Add(candidate.Path, true) {
				skipped = append(skipped, skippedHome(candidate.Path, err))
			}
		}
	}
	return homes, skipped, found, merged
}

// dropReachedRoots drops each candidate that reaching turned into the image
// root itself, or on a live run (root "") into a drive root, which is never
// a home; the skip list still applies to the others by their user name, the
// leaf of the path as written. A path spelled otherwise than root is root
// only when same says so.
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

// wantedUser reports whether one of the -u names names user, ignoring case
// for every letter (strings.EqualFold). Path keys fold ASCII only because
// merging two names NTFS keeps apart would lose evidence; a -u name that
// matches too much only selects one more home, so here every letter folds.
// A non-empty list naming nobody (",") keeps no home.
func wantedUser(names []string, user string) bool {
	return slices.ContainsFunc(names, func(name string) bool { return strings.EqualFold(name, user) })
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

// skippedHome describes a candidate home whose entry could not be looked at,
// with the text of err, Denied when its parent lists it. On NTFS whoever may
// list a directory may read its entries' attributes, so the usual case is a
// candidate that is gone or whose parent cannot be listed; Denied covers an
// entry that changed between the two looks or lies on another file system (a
// network share decides for itself), and the error_read row of such a home.
func skippedHome(path string, err error) Skipped {
	return Skipped{Path: path, Reason: err.Error(), Denied: parentLists(path)}
}
