//go:build unix

package collect

import "path/filepath"

// absoluteProject returns PROJECT, a -p path, made absolute: an absolute
// path as given, a relative one joined to CWD with . and .. taken away by
// name, so one relative spelling names one path whether or not it exists.
func absoluteProject(cwd, project string) string {
	if filepath.IsAbs(project) {
		return project
	}
	return filepath.Join(cwd, project)
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
