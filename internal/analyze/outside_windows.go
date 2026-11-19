package analyze

import (
	"io/fs"
	"path/filepath"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// absolute returns path made absolute as the command's own file calls take
// it. Windows removes "." and ".." by name before it looks at the disk, so
// the text is cleaned. A path with a drive and a root, or a UNC path, is
// what osfile opens: syspath.Extended's form, given back by syspath.Opened
// (which keeps a name ending in a dot or a space). Any other path (relative,
// rooted without a drive such as \case\out, or on a drive without a root
// such as D:out) reaches Windows as written, which places it against the
// current directory of its drive: filepath.Abs asks Windows the same
// question through syscall.FullPath, GetFullPathNameW (abs in
// path/filepath/path_windows.go).
func absolute(path string) (string, error) {
	switch {
	case !syspath.IsAbs(path):
		return filepath.Abs(path)
	case syspath.Extended(path) == path:
		// Already in the \\?\ or \\.\ form, which osfile passes on as it is.
		return path, nil
	}
	return syspath.ToNative(syspath.Opened(path)), nil
}

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
