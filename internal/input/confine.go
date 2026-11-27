package input

import (
	"errors"
	"io/fs"
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

// PlainName reports whether name can stand as one component of a path
// below a directory: it is not empty, "." or "..", and holds no slash, no
// NUL and nothing this platform reads as more than a name (on Windows a
// backslash or a colon).
func PlainName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, "/\x00") && memberNameAllowed(name)
}

// checkElement returns errOutside when element is not a PlainName.
func checkElement(element string) error {
	if !PlainName(element) {
		return errOutside
	}
	return nil
}

// lookup is how below looks at the entries on the way down. lstat, when
// set, stands in for osfile.Lstat, so a test can count the looks; list,
// when set, lists a directory to find a missing name in another ASCII case
// (osfile.Spellings), and kept, when set, keeps each listing made.
type lookup struct {
	lstat func(path string) (fs.FileInfo, error)
	list  func(directory string) osfile.Spellings
	kept  map[string]osfile.Spellings
}

// below is the one way a path derived from the evidence's text (a manifest
// field, an archive member's name, a name a parser read) becomes a disk
// path. Each element must be one plain component (checkElement); they are
// joined below base, and the components on the way down are looked at with
// lstat, base itself too when checkBase is set: a link at any of them is
// errThroughLink, never followed. When look can list, a component that does
// not exist is taken in the spelling entryBelow finds, the last one too,
// which is otherwise not looked at; the last is never refused, so the
// caller decides what it may be. A component that does not exist ends the
// look, since nothing can be read through it.
func below(base string, elements []string, checkBase bool, look lookup) (string, error) {
	if !plainParts(elements) || base == "" {
		return "", errOutside
	}
	if look.lstat == nil {
		look.lstat = osfile.Lstat
	}
	if len(elements) == 0 {
		if info, err := look.lstat(base); checkBase && err == nil && isLink(info) {
			return "", errThroughLink
		}
		return base, nil
	}
	if checkBase {
		if info, err := look.lstat(base); err != nil {
			return joinPath(base, elements...), nil
		} else if isLink(info) {
			return "", errThroughLink
		}
	}
	path := base
	for i, element := range elements {
		last := i == len(elements)-1
		if last && look.list == nil {
			return filepath.Join(path, element), nil
		}
		next, info, err := look.entryBelow(path, element)
		switch {
		case last:
			return next, nil
		case err != nil:
			return joinPath(next, elements[i+1:]...), nil
		case isLink(info):
			return "", errThroughLink
		}
		path = next
	}
	return path, nil
}

// entryBelow joins element below directory and looks at that entry. When it
// does not exist and look can list, the directory's single entry equal to
// element under ASCII fold is looked at instead, spelled as the listing
// gives it.
func (look lookup) entryBelow(directory, element string) (string, fs.FileInfo, error) {
	path := filepath.Join(directory, element)
	info, err := look.lstat(path)
	if look.list == nil || !errors.Is(err, fs.ErrNotExist) {
		return path, info, err
	}
	spelled := look.spelling(directory, element)
	if spelled == element {
		return path, info, err
	}
	path = filepath.Join(directory, spelled)
	info, err = look.lstat(path)
	return path, info, err
}

// spelling returns element as the listing of directory spells it, from the
// kept listing when there is one.
func (look lookup) spelling(directory, element string) string {
	if look.kept == nil {
		return look.list(directory).Spelling(element)
	}
	spellings, ok := look.kept[directory]
	if !ok {
		spellings = look.list(directory)
		look.kept[directory] = spellings
	}
	return spellings.Spelling(element)
}

// Below returns names joined below base, a directory the collection or the
// parser kit gave, and true; or false when a name is not one plain
// component or the way down passes through a link, base itself included.
// A name that does not exist is found in another ASCII case as below finds
// it, from a listing made for this call alone. The last name is left for
// the reader, which never follows it.
func Below(base string, names ...string) (string, bool) {
	path, err := below(base, names, true, lookup{list: osfile.ListSpellings})
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
// the analyst chose. A manifest path names the file as the collection
// archived it, so it is never respelled and its last element not looked at.
func (c *Collection) inside(elements ...string) (string, error) {
	return below(c.Root, elements, false, lookup{lstat: c.lstat})
}
