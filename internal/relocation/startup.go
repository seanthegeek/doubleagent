package relocation

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/stage"
)

// ListStartupFiles returns the regular files under base ("" for the root)
// that the table's globs of scope match, in table order and each glob's
// matches sorted, as base + "/" + match; a system glob's leading `/` is
// dropped, so it is taken under base. A symlink or other reparse point is
// never listed, nor a file under a symlinked directory or junction that
// resolves outside base, nor a path holding a newline, which would forge a
// line of the sh collector's file list. check is the image root's (nil on a
// live run): the expansion looks at nothing behind a link out of the root,
// and each such link is returned once in outOfRoot.
func ListStartupFiles(base, scope string, table []catalog.StartupFile, check glob.Check,
) (files, outOfRoot []string) {
	for _, line := range table {
		if line.Scope != scope {
			continue
		}
		paths, refused := glob.Expand(base, strings.TrimPrefix(line.Glob, "/"), check)
		for _, refusal := range refused {
			if !slices.Contains(outOfRoot, refusal.Link) {
				outOfRoot = append(outOfRoot, refusal.Link)
			}
		}
		for _, path := range paths {
			if strings.Contains(path, "\n") {
				continue
			}
			meta, err := fsmeta.Lstat(path)
			if err != nil || meta.Kind != fsmeta.File || linkedParent(base, path) {
				continue
			}
			files = append(files, path)
		}
	}
	return files, outOfRoot
}

// linkedParent reports whether a symlinked directory (or, on Windows, a
// junction) between base and path, which the glob expansion followed, leads
// outside base ("" for the root) once every link in it is resolved, or cannot
// be resolved. path is base, a slash and the rest; the sh collector's
// linked_parent counts the components of base off path in the same way.
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
		if meta, err := fsmeta.Lstat(directory); err != nil || !isLink(meta) {
			continue
		}
		if root == "" {
			physicalBase, ok := physicalDirectory(base)
			if !ok {
				return true
			}
			root = physicalBase
		}
		physical, ok := physicalDirectory(directory)
		if !ok {
			return true
		}
		if _, inside := stage.Under(physical, root); !inside {
			return true
		}
	}
}

// reparsePointAttribute is FILE_ATTRIBUTE_REPARSE_POINT; Attributes is 0 on
// Unix.
const reparsePointAttribute = 0x400

// isLink reports whether meta describes a link a glob may have followed: a
// symlink, or on Windows any reparse point, as Get-LinkedParent takes one.
func isLink(meta fsmeta.Meta) bool {
	return meta.Kind == fsmeta.Symlink || meta.Attributes&reparsePointAttribute != 0
}
