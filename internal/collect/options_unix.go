//go:build unix

package collect

import (
	"path/filepath"
	"strings"
)

// absoluteProject returns PROJECT as the sh collector resolves a -p path. An
// absolute path is kept as given. A relative one is joined to CWD: an
// existing directory is cleaned (the shell's `cd` then `pwd`); anything else
// keeps its spelling, minus one leading "./".
func absoluteProject(cwd, project string) string {
	if filepath.IsAbs(project) {
		return project
	}
	joined := filepath.Join(cwd, project)
	if isDirectory(joined) {
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
