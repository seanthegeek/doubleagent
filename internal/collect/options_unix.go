//go:build unix

package collect

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// absoluteProject returns PROJECT as the sh collector resolves a -p path. An
// absolute path is kept as given. A relative one is joined to CWD: an
// existing directory is cleaned (the shell's `cd` then `pwd`); anything else
// keeps its spelling, minus one leading "./". check is the image root's (nil
// live): a join that leads out of the image root, or is not under it, is not
// tested but cleaned, whatever the analyst's host holds there, so the project
// step names it by where it lexically is and two spellings are one project.
// In image mode a path holding a `..` component is cleaned too, so the
// project step judges the same path this test did.
func absoluteProject(cwd, project string, check follow.Check) string {
	if filepath.IsAbs(project) {
		return project
	}
	joined := filepath.Join(cwd, project)
	isDir, err := follow.IsDirectory(check, joined)
	_, leaves := follow.OutOfRoot(err)
	climbs := slices.Contains(strings.Split(project, "/"), "..")
	if isDir || leaves || (check != nil && climbs) {
		return joined
	}
	return cwd + "/" + strings.TrimPrefix(project, "./")
}

// absoluteRoot returns ROOT joined to CWD (empty when ROOT is absolute) and
// cleaned, as the shell's `cd ROOT && pwd` spells it, and the path whose
// test -d decides whether it is a directory: the uncleaned join, since ..
// after a symlinked directory leaves the link's target.
func absoluteRoot(cwd, root string) (resolved, checked string) {
	if cwd == "" {
		return filepath.Clean(root), root
	}
	return filepath.Join(cwd, root), cwd + "/" + root
}

// typedRoot returns resolved: on Unix a root is not respelled, so its typed
// and resolved forms are one.
func typedRoot(_, _, resolved string) string { return resolved }

// imageCheck returns the root check of the image the options name, resolved
// as ResolveRoot will resolve it, for the relative -p paths absoluteProject
// tests before the root is resolved: nil on a live run or when no -p is
// relative. The current directory is asked for when the root is relative. A
// root that cannot be resolved refuses every path, so nothing is stat'ed
// through it; ResolveRoot then reports the root.
func imageCheck(options Options, getwd func() (string, error)) follow.Check {
	relative := slices.ContainsFunc(options.Projects, func(project string) bool {
		return project != "" && !filepath.IsAbs(project)
	})
	if options.Root == "" || !relative {
		return nil
	}
	cwd := ""
	if !filepath.IsAbs(options.Root) {
		var err error
		if cwd, err = getwd(); err != nil {
			return func(path string) (follow.Refusal, bool) {
				return follow.Refusal{Link: path, Kind: follow.Unverified}, true
			}
		}
	}
	resolved, _ := absoluteRoot(cwd, options.Root)
	if liveRoot(resolved) {
		return nil
	}
	return walk.NewRootCheck(resolved).Expansion()
}
