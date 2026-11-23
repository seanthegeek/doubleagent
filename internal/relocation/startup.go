package relocation

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/stage"
)

// ListStartupFiles returns the regular files under base ("" for the root)
// that the table's globs of scope match, in table order (by glob, for a table
// catalog.Parse read) and each glob's matches sorted, as base + "/" + match;
// a system glob's leading `/` is dropped, so it is taken under base. A
// symlink or other reparse point is never listed, nor a file under a
// symlinked directory or junction that resolves outside base; a path holding
// a newline is listed like any other. check is the image root's (nil on a
// live run): the expansion looks at nothing behind a link out of the root,
// and each such refusal is returned once in outOfRoot.
func ListStartupFiles(base, scope string, table []catalog.StartupFile, check glob.Check,
	opener *fsmeta.Opener,
) (files []string, outOfRoot []follow.Refusal) {
	for _, line := range table {
		if line.Scope != scope {
			continue
		}
		paths, refused := glob.Expand(base, strings.TrimPrefix(line.Glob, "/"), check, opener)
		for _, refusal := range refused {
			if !slices.ContainsFunc(outOfRoot, func(known follow.Refusal) bool {
				return known.Link == refusal.Link
			}) {
				outOfRoot = append(outOfRoot, refusal.Refusal)
			}
		}
		for _, path := range paths {
			meta, err := fsmeta.Lstat(path)
			if err != nil || meta.Kind != fsmeta.File || linkedParent(base, path, check) {
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
// be resolved. path is base, a slash and the rest; the components of base
// are counted off path.
// check is the image root's (nil live); a link it refuses is not resolved.
func linkedParent(base, path string, check glob.Check) bool {
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
		if meta, err := fsmeta.Lstat(directory); err != nil || !meta.IsLink() {
			continue
		}
		if root == "" {
			physicalBase, ok := physicalDirectory(check, base)
			if !ok {
				return true
			}
			root = physicalBase
		}
		physical, ok := physicalDirectory(check, directory)
		if !ok {
			return true
		}
		if _, inside := stage.Under(physical, root); !inside {
			return true
		}
	}
}
