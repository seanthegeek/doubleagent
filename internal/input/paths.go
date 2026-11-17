package input

import "strings"

// pathForm is a path split the way a Python pathlib path holds it: an
// anchor (a drive and a root, either possibly empty) and the components
// below it, with empty and "." components gone and ".." kept.
type pathForm struct {
	drive, root string
	parts       []string
	windows     bool
}

// String returns the path as Python's str(Path) prints it: "." for an
// empty relative path.
func (p pathForm) String() string {
	separator := "/"
	if p.windows {
		separator = `\`
	}
	if p.drive != "" || p.root != "" {
		return p.drive + p.root + strings.Join(p.parts, separator)
	}
	if len(p.parts) == 0 {
		return "."
	}
	if p.windows && windowsDrive(p.parts[0]) != "" {
		return `.\` + strings.Join(p.parts, separator)
	}
	return strings.Join(p.parts, separator)
}

// name returns the last component, "" for an anchor alone.
func (p pathForm) name() string {
	if len(p.parts) == 0 {
		return ""
	}
	return p.parts[len(p.parts)-1]
}

// parent returns the path less its last component, or itself when it has
// none.
func (p pathForm) parent() pathForm {
	if len(p.parts) == 0 {
		return p
	}
	p.parts = p.parts[: len(p.parts)-1 : len(p.parts)-1]
	return p
}

// join returns p with each element appended as Python's Path.joinpath
// does: an element with a root replaces the path (keeping the drive on
// Windows unless the element names its own).
func (p pathForm) join(elements ...string) pathForm {
	for _, element := range elements {
		next := parsePathAs(element, p.windows)
		switch {
		case next.drive != "":
			p = next
		case next.root != "":
			next.drive = p.drive
			p = next
		default:
			p.parts = append(p.parts[:len(p.parts):len(p.parts)], next.parts...)
		}
	}
	return p
}

// parsePathAs splits path with the POSIX or, when windows is set, the
// Windows rules of pathlib.
func parsePathAs(path string, windows bool) pathForm {
	if windows {
		return parseWindowsPath(path)
	}
	return parsePosixPath(path)
}

// parsePosixPath splits path as PurePosixPath does: exactly two leading
// slashes are kept as the root, one or three and more become one.
func parsePosixPath(path string) pathForm {
	form := pathForm{}
	if strings.HasPrefix(path, "/") {
		form.root = "/"
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			form.root = "//"
		}
	}
	form.parts = components(path, "/")
	return form
}

// parseWindowsPath splits path as PureWindowsPath does: slashes become
// backslashes, then a drive (`C:`, `\\server\share` or a `\\?\` device) and
// a root are taken off the front, as ntpath.splitroot finds them.
func parseWindowsPath(path string) pathForm {
	path = strings.ReplaceAll(path, "/", `\`)
	form := pathForm{windows: true}
	rest := path
	switch {
	case strings.HasPrefix(path, `\\`):
		form.drive, form.root, rest = splitWindowsShare(path)
	case strings.HasPrefix(path, `\`):
		form.root, rest = `\`, path[1:]
	case windowsDrive(path) != "":
		form.drive, rest = path[:2], path[2:]
		if strings.HasPrefix(rest, `\`) {
			form.root, rest = `\`, rest[1:]
		}
	}
	form.parts = components(rest, `\`)
	return form
}

// splitWindowsShare splits a path starting with two backslashes into the
// share or device that is its drive, its root and the rest; a share whose
// path has no root gets one, as pathlib gives it.
func splitWindowsShare(path string) (drive, root, rest string) {
	start := 2
	if len(path) >= 8 && strings.EqualFold(path[:8], `\\?\UNC\`) {
		start = 8
	}
	first := strings.IndexByte(path[start:], '\\')
	if first < 0 {
		return path, "", ""
	}
	first += start
	second := strings.IndexByte(path[first+1:], '\\')
	if second < 0 {
		drive = path
	} else {
		second += first + 1
		drive, root, rest = path[:second], `\`, path[second+1:]
	}
	if root == "" && !strings.HasSuffix(drive, `\`) {
		pieces := strings.Split(drive, `\`)
		if len(pieces) == 4 && pieces[2] != "?" && pieces[2] != "." {
			root = `\`
		}
	}
	return drive, root, rest
}

// windowsDrive returns the `X:` drive that starts path, or "".
func windowsDrive(path string) string {
	if len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]) {
		return path[:2]
	}
	return ""
}

// isASCIILetter reports whether character is an ASCII letter.
func isASCIILetter(character byte) bool {
	return ('a' <= character && character <= 'z') || ('A' <= character && character <= 'Z')
}

// components splits text at separator, dropping empty and "." parts.
func components(text, separator string) []string {
	var parts []string
	for _, part := range strings.Split(text, separator) {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

// cleanPath returns path as str(Path(path)) prints it on this platform.
func cleanPath(path string) string {
	return parsePath(path).String()
}

// joinPath returns str(Path(base).joinpath(elements...)) on this platform.
func joinPath(base string, elements ...string) string {
	return parsePath(base).join(elements...).String()
}
