// Package projects finds the project directories named in the agents' state
// under each home, reproducing the sh collector's discover_projects,
// merge_project_owners and keep_project_dirs: the readers copy its grep and
// sed pipelines, line by line, rather than parse JSON, YAML or TOML, so both
// collectors find the same projects in the same order.
package projects

import (
	"context"
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
// --inventory) names files not to read; outOfScope (nil live) names paths
// leaving the image root, behind which nothing is read, each such link
// returned once as a Refusal; opener reads every file. Only a cancelled ctx
// is an error.
func Discover(ctx context.Context, homes []users.Home, root string, extra []string,
	dropSecret func(relative string) bool, outOfScope RootExit, opener *fsmeta.Opener,
	reach func(path string) string,
) ([]Project, []Refusal, error) {
	root = strings.TrimSuffix(root, "/")
	userList := userListText(homes)
	rules := platformRules()
	var found []Project
	var refused []Refusal
	for _, line := range strings.Split(userList, "\n") {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		user, home, _ := strings.Cut(line, ":")
		if home == "" {
			continue
		}
		scan := &homeScan{ctx: ctx, root: root, user: user, home: home, rules: rules,
			dropSecret: dropSecret, outOfScope: outOfScope, opener: opener}
		scan.claudeCodeSources()
		scan.cliSessionSources()
		scan.geminiSources()
		scan.agentSessionSources()
		scan.configFileSources()
		scan.editorSources()
		for _, path := range scan.paths() {
			found = append(found, Project{Owner: user, Path: path})
		}
		refused = append(refused, scan.refused...)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, path := range strings.Split(strings.Join(extra, "\n"), "\n") {
		if syspath.IsAbs(path) {
			found = append(found, Project{Owner: projectOwner(userList, path), Path: path})
		}
	}
	if reach != nil {
		for i := range found {
			found[i].Path = reach(found[i].Path)
		}
	}
	same := follow.SameEntry(outOfScope)
	return keepProjectDirs(mergeProjectOwners(found, same), userList, outOfScope, same), refused,
		nil
}

// userListText is the sh collector's USER_LIST: one user:home line per home.
// Discovery reads the homes back from this text, as sh does, so a home with
// a newline in its path splits into lines here as it does there.
func userListText(homes []users.Home) string {
	lines := make([]string, len(homes))
	for i, home := range homes {
		lines[i] = home.User + ":" + home.Path
	}
	return strings.Join(lines, "\n")
}

// projectOwner is get_project_owner: the user of the first USER_LIST line
// whose home, followed by `/`, starts path, or "" when none does. On Windows
// a home spelling path's start byte for byte wins; failing one, the first
// home matching ignoring ASCII case, as Get-ProjectOwner compares, owns it:
// the owner only labels the project's rows, so a wrong one loses nothing. A
// line with no colon is its own user and home, as awk's index and $1 give.
func projectOwner(userList, path string) string {
	folded, foldedFound := "", false
	for _, line := range strings.Split(userList, "\n") {
		user, home, found := strings.Cut(line, ":")
		if !found {
			home = line
		}
		if strings.HasPrefix(path, home+"/") {
			return user
		}
		if !foldedFound && syspath.HasPrefix(path, home+"/") {
			folded, foldedFound = user, true
		}
	}
	return folded
}

// mergeProjectOwners is merge_project_owners: one entry per path, in order
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

// keepProjectDirs is keep_project_dirs: the projects whose path is a
// directory and is not the home of a USER_LIST line, which sh tests as the
// text after some colon ending a line, ignoring case on Windows. A path
// outOfScope (when not nil) names skips the directory test and is never
// stat'ed. A home spelled otherwise only in ASCII case is the project's own
// directory only when same says so (listedHome).
func keepProjectDirs(projects []Project, userList string, outOfScope RootExit,
	same syspath.SameEntry,
) []Project {
	listed := "\n" + userList
	var kept []Project
	for _, project := range projects {
		isDir, err := follow.IsDirectory(outOfScope, project.Path)
		if _, leaves := follow.OutOfRoot(err); !leaves && !isDir {
			continue
		}
		if listedHome(listed, project.Path, same) {
			continue
		}
		kept = append(kept, project)
	}
	return kept
}

// listedHome reports whether path follows a colon and ends a line of
// listed, the USER_LIST text after a newline: byte for byte, or on Windows
// ignoring ASCII case where same says the text there and path are one entry.
func listedHome(listed, path string, same syspath.SameEntry) bool {
	if strings.Contains(listed, ":"+path+"\n") || strings.HasSuffix(listed, ":"+path) {
		return true
	}
	// syspath.Fold changes no byte's position, so offsets in the folded
	// text are offsets in listed.
	folded, needle := syspath.Fold(listed), ":"+syspath.Fold(path)
	for start := 0; ; {
		position := strings.Index(folded[start:], needle)
		if position < 0 {
			return false
		}
		position += start
		end := position + len(needle)
		if (end == len(folded) || folded[end] == '\n') &&
			syspath.SameAs(listed[position+1:end], path, same) {
			return true
		}
		start = position + 1
	}
}
