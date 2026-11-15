package relocation

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// ListStartupFiles returns the regular files under base ("" for the root)
// that the table's globs of scope match, in table order and each glob's
// matches sorted, as base + "/" + match; a system glob's leading `/` is
// dropped, so it is taken under base. A symlink is never listed, nor a file
// under a symlinked directory that resolves outside base, nor a path holding
// a newline, which would forge a line of the sh collector's file list.
func ListStartupFiles(base, scope string, table []catalog.StartupFile) []string {
	var files []string
	for _, line := range table {
		if line.Scope != scope {
			continue
		}
		for _, path := range glob.Expand(base, strings.TrimPrefix(line.Glob, "/")) {
			if strings.Contains(path, "\n") {
				continue
			}
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() || linkedParent(base, path) {
				continue
			}
			files = append(files, path)
		}
	}
	return files
}

// linkedParent reports whether a symlinked directory between base and path,
// which the glob expansion followed, leads outside base ("" for the root)
// once every link in it is resolved, or cannot be resolved. path is base, a
// slash and the rest; the sh collector's linked_parent counts the components
// of base off path in the same way.
func linkedParent(base, path string) bool {
	rest := path
	for range strings.Count(base+"/", "/") {
		if _, after, found := strings.Cut(rest, "/"); found {
			rest = after
		}
	}
	directory, root := base, ""
	for {
		segment, after, found := strings.Cut(rest, "/")
		if !found {
			return false
		}
		directory, rest = directory+"/"+segment, after
		if info, err := os.Lstat(directory); err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if root == "" {
			physicalBase, ok := physicalDirectory(base)
			if !ok {
				return true
			}
			root = strings.TrimSuffix(physicalBase, "/") + "/"
		}
		physical, ok := physicalDirectory(directory)
		if !ok || !strings.HasPrefix(physical+"/", root) {
			return true
		}
	}
}

// physicalDirectory returns the physical path of directory ("" for the
// root), every link resolved, as `cd -P DIR && pwd -P` prints it; ok is false
// where cd would fail: no such directory, or no search permission on it.
func physicalDirectory(directory string) (physical string, ok bool) {
	if directory == "" {
		directory = "/"
	}
	physical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(physical)
	if err != nil || !info.IsDir() || !canEnter(physical) {
		return "", false
	}
	return physical, true
}
