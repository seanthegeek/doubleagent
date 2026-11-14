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
