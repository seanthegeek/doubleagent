package input

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The reasons a path built from the evidence's text is refused.
var (
	// errOutside is a name that is not one plain component: it is empty,
	// "." or "..", or holds a slash, a NUL or a character this platform
	// reads as more than a name (on Windows a backslash or a colon).
	errOutside = errors.New("outside the collection")
	// errThroughLink is a path that would pass through a symbolic link (on
	// Windows, any reparse point) on its way down.
	errThroughLink = errors.New("through a symbolic link, not followed")
)

// checkElement returns errOutside when element cannot stand as one
// component of a path below a directory.
func checkElement(element string) error {
	if element == "" || element == "." || element == ".." ||
		strings.ContainsAny(element, "/\x00") || !memberNameAllowed(element) {
		return errOutside
	}
	return nil
}

// below is the one way a path derived from the evidence's text (a manifest
// field, an archive member's name, a name a parser read) becomes a disk
// path. Each element must be one plain component (checkElement); they are
// joined below base, and the components on the way down are looked at with
// lstat, base itself too when checkBase is set: a link at any of them is
// errThroughLink, never followed. The last element is not looked at, so
// the caller decides what it may be; a component that does not exist ends
// the look, since nothing can be read through it.
func below(base string, elements []string, checkBase bool) (string, error) {
	for _, element := range elements {
		if err := checkElement(element); err != nil {
			return "", err
		}
	}
	if base == "" {
		return "", errOutside
	}
	path := base
	if len(elements) == 0 {
		return path, nil
	}
	for i, element := range elements {
		if i > 0 || checkBase {
			info, err := osfile.Lstat(path)
			if err != nil {
				return joinPath(path, elements[i:]...), nil
			}
			if isLink(info) {
				return "", errThroughLink
			}
		}
		path = filepath.Join(path, element)
	}
	return path, nil
}

// Below returns names joined below base, a directory the collection or the
// parser kit gave, and true; or false when a name is not one plain
// component or the way down passes through a link, base itself included.
// The last name is left for the reader, which never follows it.
func Below(base string, names ...string) (string, bool) {
	path, err := below(base, names, true)
	return path, err == nil
}

// parentPath returns the directory holding name, given path, the result of
// joining a plain component name below that directory as below does: a
// slice of path, the separator before name dropped unless it is the root's
// own.
func parentPath(path, name string) string {
	parent := path[:len(path)-len(name)]
	if len(parent) > len(filepath.VolumeName(parent))+1 {
		parent = parent[:len(parent)-1]
	}
	return parent
}

// inside returns the root joined with elements as below does, the root
// itself not looked at: it is the analyst's own input, which may be a link
// the analyst chose.
func (c *Collection) inside(elements ...string) (string, error) {
	return below(c.Root, elements, false)
}
