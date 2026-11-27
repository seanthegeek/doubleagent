package walk

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// linkForm is a Windows link target the root check knows how to follow: the
// target in canonical form, and how it is placed. kind is relativeTarget for
// a name joined to the link's own path, rootedTarget for `/x` on the link's
// drive or share, absoluteTarget for a drive or UNC path.
type linkForm struct {
	target string
	kind   int
}

// The placements a linkForm can have.
const (
	relativeTarget = iota + 1
	rootedTarget
	absoluteTarget
)

// relativeForm accepts the name of a symbolic link marked relative: a plain
// relative path, or `\x` rooted on the link's drive, with no prefix, colon or
// doubled leading separator.
func relativeForm(name string) (linkForm, bool) {
	if name == "" || strings.ContainsRune(name, ':') || strings.HasPrefix(name, `\\`) {
		return linkForm{}, false
	}
	target := strings.ReplaceAll(name, `\`, "/")
	if len(target) > 1 {
		target = strings.TrimSuffix(target, "/")
	}
	if strings.HasPrefix(target, "/") {
		return linkForm{target: target, kind: rootedTarget}, true
	}
	return linkForm{target: target, kind: relativeTarget}, true
}

// absoluteForm accepts `\??\X:\...`, and for a symbolic link `\??\UNC\server
// \share\...`; a junction's name may hold no `.` or `..` component, since
// Windows refuses to follow such a junction.
func absoluteForm(name string, symlink bool) (linkForm, bool) {
	rest, found := strings.CutPrefix(name, `\??\`)
	if !found {
		return linkForm{}, false
	}
	parts := strings.Split(rest, `\`)
	if !symlink && (slices.Contains(parts, ".") || slices.Contains(parts, "..")) {
		return linkForm{}, false
	}
	head := parts[0]
	if len(head) == 2 && head[1] == ':' && isASCIILetter(head[0]) && len(parts) > 1 {
		return linkForm{target: strings.TrimSuffix(strings.Join(parts, "/"), "/"),
			kind: absoluteTarget}, true
	}
	if symlink && head == "UNC" && len(parts) > 2 && parts[1] != "" && parts[2] != "" {
		return linkForm{target: "//" + strings.Join(parts[1:], "/"), kind: absoluteTarget}, true
	}
	return linkForm{}, false
}

// acceptLink returns the form of the link the reparse data describes when it
// is one of a closed set, and false for anything else, which the check
// refuses: a symbolic link marked relative whose name is a plain relative
// path or `\x`; a symbolic link not marked relative whose name is `\??\X:\`
// or `\??\UNC\server\share\`; a junction whose name is `\??\X:\` with no `.`
// or `..` component. Volume GUID names, GLOBALROOT, `\Device\`, `\GLOBAL??\`,
// `\\.\`, a drive-relative `X:x`, an empty name and any other tag are not in
// the set.
func acceptLink(data fsmeta.Reparse) (linkForm, bool) {
	name := data.Substitute
	switch {
	case data.Tag == fsmeta.TagSymlink && data.Relative:
		return relativeForm(name)
	case data.Tag == fsmeta.TagSymlink:
		return absoluteForm(name, true)
	case data.Tag == fsmeta.TagMountPoint:
		return absoluteForm(name, false)
	}
	return linkForm{}, false
}
