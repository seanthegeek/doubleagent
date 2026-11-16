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
// live), then the absolute -p directories in extra, first seen first, each
// once (ignoring case on Windows) with its first non-empty owner, keeping
// directories (test -d) that are not a home. dropSecret, nil outside
// --inventory, names files, relative to their home, not to read. outOfScope,
// nil live, names paths leaving the image root: such a project is kept
// untested; behind such a link nothing is listed, probed or read, and the
// link is returned once as a Refusal. opener reads every file (nil: see
// fsmeta.Opener). Only a cancelled ctx is an error.
func Discover(ctx context.Context, homes []users.Home, root string, extra []string,
	dropSecret func(relative string) bool, outOfScope RootExit, opener *fsmeta.Opener,
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
	return keepProjectDirs(mergeProjectOwners(found), userList, outOfScope), refused, nil
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
// whose home, followed by `/`, starts path (ignoring case on Windows, as
// Get-ProjectOwner compares), or "" when none does. A line with no colon is
// its own user and home, as awk's index and $1 give.
func projectOwner(userList, path string) string {
	for _, line := range strings.Split(userList, "\n") {
		user, home, found := strings.Cut(line, ":")
		if !found {
			home = line
		}
		if syspath.HasPrefix(path, home+"/") {
			return user
		}
	}
	return ""
}

// mergeProjectOwners is merge_project_owners: one entry per path, in order
// of first appearance, owned by the first non-empty owner given for it; an
// empty path is skipped. Paths that differ only in case are one on Windows,
// as Select-UniqueProjects keys them.
func mergeProjectOwners(found []Project) []Project {
	index := map[string]int{}
	var merged []Project
	for _, project := range found {
		if project.Path == "" {
			continue
		}
		key := syspath.Fold(project.Path)
		position, seen := index[key]
		switch {
		case !seen:
			index[key] = len(merged)
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
// stat'ed.
func keepProjectDirs(projects []Project, userList string, outOfScope RootExit) []Project {
	listed := "\n" + syspath.Fold(userList)
	var kept []Project
	for _, project := range projects {
		isDir, err := follow.IsDirectory(outOfScope, project.Path)
		if _, leaves := follow.OutOfRoot(err); !leaves && !isDir {
			continue
		}
		folded := syspath.Fold(project.Path)
		if strings.Contains(listed, ":"+folded+"\n") || strings.HasSuffix(listed, ":"+folded) {
			continue
		}
		kept = append(kept, project)
	}
	return kept
}
