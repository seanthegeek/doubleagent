package users

import (
	"fmt"
	"slices"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// homeGlobs returns the home parents relative to the base, in the order the
// PowerShell collector tries them: a live run looks under the system drive,
// an image also at the Unix home parents, Users first in both.
func homeGlobs(live bool) []string {
	if live {
		return []string{"Users/*", "home/*"}
	}
	return []string{"Users/*", "home/*", "root", "var/root", "usr/home/*", "export/home/*"}
}

// Find returns the homes to collect, in enumeration order. A live run reads
// the registry's ProfileList first, then both modes add the directories
// under the home parents of root (the system drive when live). A candidate
// that is a file is dropped; one whose entry cannot be looked at is reported
// in Skipped, with Denied set when its parent lists it. Both tests read the
// entry's own attributes, as .NET's File.Exists and Directory.Exists do. The
// -u list is compared against the user name ignoring every letter's case
// (wantedUser says why). A
// ProfileList path is respelled as the disk holds it, so a home is kept once
// by its ASCII-folded path. run is not used on Windows.
func Find(live bool, root, usersFilter string, _ Runner) Report {
	var reader profileReader
	if live {
		reader = registryProfiles{}
	}
	return find(reader, root, usersFilter)
}

// find is Find with the ProfileList reader given; a nil reader means an
// image run.
func find(reader profileReader, root, usersFilter string) Report {
	root = syspath.FromNative(root)
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
	candidates = append(candidates, globHomes(syspath.SystemBase(root), reader != nil)...)
	report.Homes, report.Skipped, report.Found = filterProfiles(candidates, usersFilter)
	report.Unmatched = unmatchedUsers(userNames(usersFilter, strings.EqualFold), report.Found,
		strings.EqualFold)
	return report
}

// globHomes returns the directories under each home parent of base, each
// named by its last path segment.
func globHomes(base string, live bool) []Home {
	var homes []Home
	for _, pattern := range homeGlobs(live) {
		for _, match := range glob.Expand(base, pattern) {
			homes = append(homes, Home{match[strings.LastIndexByte(match, '/')+1:], match})
		}
	}
	return homes
}

// filterProfiles applies the skip list, the file test, the -u list and the
// directory test to the candidates in order, and drops repeats. One look at
// each entry's own attributes serves both tests: an entry without the
// directory attribute is a file, as System.IO.File.Exists reads it, so a
// file symlink is a file whatever it leads to; one with it is a home, as
// Directory.Exists reads it, so a junction or directory symlink whose target
// is gone is collected, with its error_read row, rather than skipped.
func filterProfiles(candidates []Home, usersFilter string) ([]Home, []Skipped, []string) {
	names := userNames(usersFilter, strings.EqualFold)
	var homes []Home
	var skipped []Skipped
	var found []string
	seenHomes, seenSkipped := map[string]bool{}, map[string]bool{}
	for _, candidate := range candidates {
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
		key := syspath.Fold(candidate.Path)
		if err == nil {
			if !seenHomes[key] {
				homes = append(homes, candidate)
			}
			seenHomes[key] = true
		} else {
			if !seenSkipped[key] {
				skipped = append(skipped, skippedHome(candidate.Path, err))
			}
			seenSkipped[key] = true
		}
	}
	return homes, skipped, found
}

// wantedUser reports whether one of the -u names names user, ignoring case
// for every letter (strings.EqualFold, Unicode simple folding). Path keys fold ASCII only because merging two
// names NTFS keeps apart would lose evidence; a -u name that matches too much
// can only select one more home, so this one comparison folds every letter.
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
		return 0, fmt.Errorf("lstat %s: no file attributes", syspath.ToNative(path))
	}
	return data.FileAttributes, nil
}

// skippedHome describes a candidate home whose entry could not be looked at,
// with the text of err, Denied when its parent lists it. On NTFS whoever may
// list a directory may read the attributes of its entries, so a look fails
// on a listed entry only when the entry changes between the two looks or
// lives on another file system (a network share decides for itself); a
// candidate that is gone, or whose parent cannot be listed, is the case NTFS
// gives. Denied stays for those other cases and for the error_read row the
// PowerShell collector writes for such a home.
func skippedHome(path string, err error) Skipped {
	return Skipped{Path: path, Reason: err.Error(), Denied: parentLists(path)}
}

// parentLists reports whether the directory holding path lists an entry of
// that name, ignoring case.
func parentLists(path string) bool {
	separator := strings.LastIndexByte(path, '/')
	if separator < 0 {
		return false
	}
	parent, leaf := path[:separator], path[separator+1:]
	if len(parent) == 2 && parent[1] == ':' {
		parent += "/" // a bare drive names the drive's current directory
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
