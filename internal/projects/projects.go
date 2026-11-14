// Package projects finds the project directories named in the agents' state
// under each home, reproducing the sh collector's discover_projects,
// merge_project_owners and keep_project_dirs: the readers copy its grep and
// sed pipelines, line by line, rather than parse JSON, YAML or TOML, so both
// collectors find the same projects in the same order.
package projects

import (
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/users"
)

// Project is a project directory and the user it is collected for: the user
// whose home's state named it, or for a -p directory the user whose home
// holds it, which may be empty.
type Project struct {
	Owner string
	Path  string
}

// Discover returns the projects named in the state of each home, then the
// -p directories in extra (used as given), in the order their paths are
// first seen, each once with its first non-empty owner, keeping directories
// (test -d) that are not a home. A discovered value is kept when it starts
// with `/`, `\/` undone and root ("" or "/" live) prefixed. dropSecret, nil
// outside --inventory, names the files, relative to their home, not to read.
// outOfScope, nil live, names the paths that leave the image root, kept
// without the directory test, which would answer for the analyst's host.
// Only a cancelled ctx is an error.
func Discover(ctx context.Context, homes []users.Home, root string, extra []string,
	dropSecret func(relative string) bool, outOfScope func(path string) bool,
) ([]Project, error) {
	root = strings.TrimSuffix(root, "/")
	userList := userListText(homes)
	var found []Project
	for _, line := range strings.Split(userList, "\n") {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		user, home, _ := strings.Cut(line, ":")
		if home == "" {
			continue
		}
		scan := &homeScan{ctx: ctx, root: root, home: home, dropSecret: dropSecret}
		scan.claudeCodeSources()
		scan.cliSessionSources()
		scan.geminiSources()
		scan.agentSessionSources()
		scan.configFileSources()
		scan.editorSources()
		for _, value := range scan.values() {
			found = append(found, Project{Owner: user, Path: root + value})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, path := range strings.Split(strings.Join(extra, "\n"), "\n") {
		if strings.HasPrefix(path, "/") {
			found = append(found, Project{Owner: projectOwner(userList, path), Path: path})
		}
	}
	return keepProjectDirs(mergeProjectOwners(found), userList, outOfScope), nil
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
// whose home, followed by `/`, starts path, or "" when none does. A line with
// no colon is its own user and home, as awk's index and $1 give.
func projectOwner(userList, path string) string {
	for _, line := range strings.Split(userList, "\n") {
		user, home, found := strings.Cut(line, ":")
		if !found {
			home = line
		}
		if strings.HasPrefix(path, home+"/") {
			return user
		}
	}
	return ""
}

// mergeProjectOwners is merge_project_owners: one entry per path, in order
// of first appearance, owned by the first non-empty owner given for it; an
// empty path is skipped.
func mergeProjectOwners(found []Project) []Project {
	index := map[string]int{}
	var merged []Project
	for _, project := range found {
		if project.Path == "" {
			continue
		}
		position, seen := index[project.Path]
		switch {
		case !seen:
			index[project.Path] = len(merged)
			merged = append(merged, project)
		case merged[position].Owner == "":
			merged[position].Owner = project.Owner
		}
	}
	return merged
}

// keepProjectDirs is keep_project_dirs: the projects whose path is a
// directory and is not the home of a USER_LIST line, which sh tests as the
// text after some colon ending a line. A path outOfScope (when not nil)
// names skips the directory test and is never stat'ed.
func keepProjectDirs(projects []Project, userList string,
	outOfScope func(path string) bool) []Project {
	listed := "\n" + userList
	var kept []Project
	for _, project := range projects {
		skipTest := outOfScope != nil && outOfScope(project.Path)
		if !skipTest && !isDirectory(project.Path) {
			continue
		}
		if strings.Contains(listed, ":"+project.Path+"\n") ||
			strings.HasSuffix(listed, ":"+project.Path) {
			continue
		}
		kept = append(kept, project)
	}
	return kept
}
