package projects

import (
	"context"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// Project is a project directory and the user it is collected for: the user
// whose home's state named it, or for a -p directory the user whose home
// holds it, which may be empty.
type Project struct {
	Owner string
	Path  string
}

// Refusal is a link in User's home that leads out of the image root (a
// symlink or, on Windows, a junction, possibly a source itself): discovery
// listed, stat'ed and read nothing behind it. It names no source, since the
// names behind the link are the analyst's host's, not the image's.
type Refusal struct {
	User string
	follow.Refusal
}

// RootExit reports whether path leads out of the image root through a link,
// and the link where it does (walk.LeavesRootAt): the check the follow
// functions take.
type RootExit = follow.Check

// Discover returns the canonical paths of the projects named in the state of
// each home (localProjectPath under the host's rules and root, "" or "/"
// live), then the absolute -p directories in extra, each path first put
// through reach (walk.RootCheck.Reached; nil: none), then each once, first
// seen first (ignoring case on Windows) with its first non-empty owner,
// keeping directories that are not a home. dropSecret (nil outside
// --inventory) names files not to read, by home and path relative to it; outOfScope (nil live) names paths
// leaving the image root, behind which nothing is read, each such link
// returned once as a Refusal; opener reads every file. Only a cancelled ctx
// is an error.
func Discover(ctx context.Context, homes []users.Home, root string, extra []string,
	dropSecret func(home, relative string) bool, outOfScope RootExit, opener *fsmeta.Opener,
	reach func(path string) string,
) ([]Project, []Refusal, error) {
	root = strings.TrimSuffix(root, "/")
	rules := platformRules()
	var found []Project
	var refused []Refusal
	for _, home := range homes {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if home.Path == "" {
			continue
		}
		scan := &homeScan{ctx: ctx, root: root, user: home.User, home: home.Path, rules: rules,
			dropSecret: dropSecret, outOfScope: outOfScope, opener: opener}
		scan.claudeCodeSources()
		scan.cliSessionSources()
		scan.geminiSources()
		scan.agentSessionSources()
		scan.configFileSources()
		scan.editorSources()
		for _, path := range scan.paths() {
			found = append(found, Project{Owner: home.User, Path: path})
		}
		refused = append(refused, scan.refused...)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, path := range extra {
		if syspath.IsAbs(path) {
			found = append(found, Project{Owner: projectOwner(homes, path), Path: path})
		}
	}
	if reach != nil {
		for i := range found {
			found[i].Path = reach(found[i].Path)
		}
	}
	same := follow.SameEntry(outOfScope)
	return keepProjectDirs(mergeProjectOwners(found, same), homes, outOfScope, same), refused, nil
}

// projectOwner returns the user of the first home that, followed by `/`,
// starts path, or "" when none does. Each home is a value, whatever bytes its
// path holds. On Windows a home spelling path's start byte for byte wins;
// failing one, the first home matching ignoring ASCII case owns it: the
// owner only labels the project's rows, so a wrong one loses nothing.
func projectOwner(homes []users.Home, path string) string {
	folded, foldedFound := "", false
	for _, home := range homes {
		if strings.HasPrefix(path, home.Path+"/") {
			return home.User
		}
		if !foldedFound && syspath.HasPrefix(path, home.Path+"/") {
			folded, foldedFound = home.User, true
		}
	}
	return folded
}

// mergeProjectOwners returns one entry per path, in order
// of first appearance, owned by the first non-empty owner given for it; an
// empty path is skipped. Two paths are one project when same says so
// (syspath.Entries): on Windows, two spellings of one directory, but not two
// directories of a case-sensitive directory.
func mergeProjectOwners(found []Project, same syspath.SameEntry) []Project {
	index := syspath.NewEntries[int](same)
	var merged []Project
	for _, project := range found {
		if project.Path == "" {
			continue
		}
		position, _, seen := index.Find(project.Path)
		switch {
		case !seen:
			index.Put(project.Path, len(merged))
			merged = append(merged, project)
		case merged[position].Owner == "":
			merged[position].Owner = project.Owner
		}
	}
	return merged
}

// keepProjectDirs returns the projects whose path is a directory and is not
// one of homes. A path outOfScope (when not nil) names skips the directory
// test and is never stat'ed. A home spelled otherwise only in ASCII case is
// the project's own directory only when same says so (syspath.SameAs).
func keepProjectDirs(projects []Project, homes []users.Home, outOfScope RootExit,
	same syspath.SameEntry,
) []Project {
	var kept []Project
	for _, project := range projects {
		isDir, err := follow.IsDirectory(outOfScope, project.Path)
		if _, leaves := follow.OutOfRoot(err); !leaves && !isDir {
			continue
		}
		if slices.ContainsFunc(homes, func(home users.Home) bool {
			return syspath.SameAs(home.Path, project.Path, same)
		}) {
			continue
		}
		kept = append(kept, project)
	}
	return kept
}
