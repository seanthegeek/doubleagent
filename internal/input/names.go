package input

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// inputNames maps the absolute paths the input is opened by back to the
// names the analyst gave. What the commands print (a problem line, a note,
// a home, an error) names the input as given, cleaned; the absolute form,
// which holds the analyst's working directory, is only for opening files.
type inputNames struct {
	// pairs holds each absolute directory or file with its printed name.
	pairs [][2]string
}

// add maps the absolute path disk, and every path below it, to shown.
func (n *inputNames) add(disk, shown string) {
	n.pairs = append(n.pairs, [2]string{disk, shown})
}

// path returns path with the first mapped absolute path that is path or
// one of its directories replaced by its printed name, or path unchanged
// when none is.
func (n *inputNames) path(path string) string {
	if n == nil {
		return path
	}
	for _, pair := range n.pairs {
		if path == pair[0] {
			return pair[1]
		}
		rest, found := strings.CutPrefix(path, pair[0])
		switch {
		case !found:
		case strings.HasSuffix(pair[0], string(filepath.Separator)):
			return filepath.Join(pair[1], rest)
		case strings.HasPrefix(rest, string(filepath.Separator)):
			return filepath.Join(pair[1], rest)
		}
	}
	return path
}

// error returns err with the path of the *fs.PathError it holds, if any,
// given its printed name. The error is one an operating-system call of
// this open made, so it is changed in place.
func (n *inputNames) error(err error) error {
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		pathError.Path = n.path(pathError.Path)
	}
	return err
}

// shownParent returns the printed name of the directory holding the path
// printed as shown, a cleaned path: "." stands for the working directory,
// so its parent is "..", and the parent of ".." is "../..".
func shownParent(shown string) string {
	if base := filepath.Base(shown); base == "." || base == ".." {
		return filepath.Join(shown, "..")
	}
	return filepath.Dir(shown)
}
