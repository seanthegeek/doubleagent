package analyze

import (
	"io/fs"
	"path/filepath"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// placed returns path made absolute as the command's own file calls take
// it: input.Absolute, the rule the input is read by, so the output and the
// input are placed by one function. Every Windows file call takes the
// \\?\ form, which has no length limit.
func placed(path string) (string, error) { return input.Absolute(path) }

// followLink refuses to follow the link at path: on Windows the check
// follows no link, so a path through one is refused as untold. Every
// Windows file call goes through osfile in its \\?\ form, and
// filepath.EvalSymlinks does not.
func followLink(path string) (string, error) {
	return "", &fs.PathError{Op: "resolve", Path: path, Err: errNotFollowed}
}

// outputParent returns the directory holding path's last element by its
// text, as Windows resolves it, or "" for a path of one element.
func outputParent(path string) string {
	if parent := filepath.Dir(path); parent != "." {
		return parent
	}
	return ""
}

// belowGiven returns the path of name in directory. Windows resolves a ".."
// by the text before it, so filepath.Join, which also keeps a drive-relative
// C: as it is, names the directory that was listed.
func belowGiven(directory, name string) string { return filepath.Join(directory, name) }
