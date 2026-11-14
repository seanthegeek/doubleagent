package users

import (
	"os"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/glob"
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
// that is a file is dropped; one that is not a directory is reported in
// Skipped, with Denied set when its parent lists it. The -u list is compared
// case-insensitively against the user name. A home is kept once, by the case-
// folded path. run is not used on Windows.
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
	if root == "/" {
		root = ""
	}
	var report Report
	var candidates []Home
	if reader != nil {
		report.Profiles = reader.Profiles()
		for _, profile := range report.Profiles {
			candidates = append(candidates, Home{profile.User, profile.Path})
		}
	}
	candidates = append(candidates, globHomes(syspath.SystemBase(root), reader != nil)...)
	report.Homes, report.Skipped = filterProfiles(candidates, usersFilter)
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
// directory test to the candidates in order, and drops repeats.
func filterProfiles(candidates []Home, usersFilter string) ([]Home, []Skipped) {
	wanted, filtered := wantedUsers(usersFilter)
	var homes []Home
	var skipped []Skipped
	seenHomes, seenSkipped := map[string]bool{}, map[string]bool{}
	for _, candidate := range candidates {
		if skipProfile(candidate.User) || isFile(candidate.Path) {
			continue
		}
		if filtered && !wanted[syspath.Fold(candidate.User)] {
			continue
		}
		key := syspath.Fold(candidate.Path)
		if isDirectory(candidate.Path) {
			if !seenHomes[key] {
				homes = append(homes, candidate)
			}
			seenHomes[key] = true
		} else {
			if !seenSkipped[key] {
				skipped = append(skipped, skippedHome(candidate.Path))
			}
			seenSkipped[key] = true
		}
	}
	return homes, skipped
}

// wantedUsers splits the -u list at commas, without trimming and dropping
// empty items, into case-folded names. filtered is true for any non-empty
// list, even one that names nobody (","), which then keeps no home.
func wantedUsers(usersFilter string) (wanted map[string]bool, filtered bool) {
	wanted = map[string]bool{}
	for _, name := range strings.Split(usersFilter, ",") {
		if name != "" {
			wanted[syspath.Fold(name)] = true
		}
	}
	return wanted, usersFilter != ""
}

// isFile reports whether path exists and is not a directory, as
// System.IO.File.Exists does; a link is followed.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// skippedHome describes a candidate home that cannot be seen as a directory:
// denied when its parent lists it, with the text of the failure to look at
// it, or "access denied" when looking at it itself works.
func skippedHome(path string) Skipped {
	reason := "access denied"
	if _, err := os.Lstat(path); err != nil {
		reason = syspath.NativeErrors(err).Error()
	}
	return Skipped{Path: path, Reason: reason, Denied: parentLists(path)}
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
	entries, err := os.ReadDir(parent)
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
