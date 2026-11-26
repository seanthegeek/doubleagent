package input

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// windowsRow reports whether a manifest row comes from a Windows host: its
// home or path is a drive or UNC path. Only such a row's paths are split at
// backslashes, since a POSIX file name may hold one.
func windowsRow(row jsonvalue.Value) bool {
	return timeline.IsWindowsPath(textOf(row.Get("home"))) || timeline.IsWindowsPath(textOf(row.Get("path")))
}

// slashForm returns a path of a Windows row with its backslashes as
// slashes, and a path of a POSIX row as it is.
func slashForm(path string, windows bool) string {
	if windows {
		return strings.ReplaceAll(path, `\`, "/")
	}
	return path
}

// relativeToHome returns path relative to home, both paths of one row, as
// a slash path without a leading slash; a path outside home is returned
// whole, less its leading slashes.
func relativeToHome(path, home string, windows bool) string {
	slashPath := slashForm(path, windows)
	slashHome := strings.TrimRight(slashForm(home, windows), "/")
	if slashHome != "" && (slashPath == slashHome || strings.HasPrefix(slashPath, slashHome+"/")) {
		return strings.TrimLeft(slashPath[len(slashHome):], "/")
	}
	return strings.TrimLeft(slashPath, "/")
}

// archiveDirFromRow returns `fs/<home>` as the collection laid the home out,
// derived from one row, or "" for a row archived at a symlink's physical
// path: the archive path must end with the home-relative path, and what is
// left must be the home's drive form or fs plus the home less a leading
// root, never the same path below another directory.
func archiveDirFromRow(row jsonvalue.Value, windows bool) string {
	home := textOf(row.Get("home"))
	archivePath := slashForm(textOf(row.Get("archive_path")), windows)
	rel := relativeToHome(textOf(row.Get("path")), home, windows)
	if rel == "" || !strings.HasSuffix(archivePath, "/"+rel) {
		return ""
	}
	archiveDir := archivePath[:len(archivePath)-len(rel)-1]
	if archiveDir == archiveDirFallback(home, "", windows) {
		return archiveDir
	}
	slashHome := slashForm(home, windows)
	if strings.HasPrefix(archiveDir, "fs/") && strings.HasSuffix(slashHome, archiveDir[len("fs"):]) {
		return archiveDir
	}
	return ""
}

// archiveDirFallback returns `fs/<home>` as `collect` lays a home out
// when no row has shown it: less the image root of an image run, and a
// Windows path as `fs/<drive letter>/<path>` (C:\Users\bob is
// fs/C/Users/bob).
func archiveDirFallback(home, imageRoot string, windows bool) string {
	slashHome := slashForm(home, windows)
	slashRoot := strings.TrimRight(slashForm(imageRoot, windows), "/")
	if slashRoot != "" && strings.HasPrefix(slashHome, slashRoot+"/") {
		return "fs/" + slashHome[len(slashRoot)+1:]
	}
	if windowsDrive(slashHome) != "" {
		return "fs/" + strings.ToUpper(slashHome[:1]) + "/" + strings.TrimLeft(slashHome[2:], "/")
	}
	return "fs" + slashHome
}

// archiveParts returns the non-empty parts of a manifest archive path, or a
// home directory below the collection, as the extraction named the file:
// split at backslashes too in a Windows row, in a collection extracted from
// a zip (extractZip splits every member name there, whatever host the row
// came from), or on a host where a backslash separates components.
func (c *Collection) archiveParts(archivePath string, windows bool) []string {
	return evidencepath.Elements(memberSeparators(archivePath), windows || c.fromZip)
}

// archivedInPlace reports whether the row's archive path is where the
// home's own directory holds its rel.
func archivedInPlace(home *Home, collection *Collection, row archivedRow) bool {
	return samePath(joinPath(collection.Root, collection.archiveParts(row.archivePath, row.windows)...),
		joinPath(home.DiskPath, evidencepath.Elements(row.rel, false)...))
}

// mapStagedPath records in the home's staged paths a row archived away from
// DiskPath/rel: the file and each directory above it, down to the symlinked
// directory that moved it (the deepest ancestor of rel among links, else
// the shared right-hand components, stopping below a directory in inPlace),
// map to their place in the archive. A rel part that is not one plain
// component, or a location inside refuses, is a problem of the row and maps
// nothing. Every key and location is a prefix of one joined string, so a
// row's work grows with its length, not with its length times its depth.
func (load *manifestLoad) mapStagedPath(home *Home, row archivedRow, links, inPlace map[string]bool) {
	relParts := evidencepath.Elements(row.rel, false)
	parts := load.collection.archiveParts(row.archivePath, row.windows)
	if len(relParts) == 0 || archivedInPlace(home, load.collection, row) {
		return
	}
	if startsOutside(row.archivePath) || !plainParts(relParts) {
		load.collection.recordProblem(row.label + ": " + errOutside.Error())
		return
	}
	equal := 0
	for equal < len(relParts) && equal < len(parts) &&
		relParts[len(relParts)-1-equal] == parts[len(parts)-1-equal] {
		equal++
	}
	relText, ends := joinedPrefixes(relParts)
	limit := min(equal, len(relParts)-1)
	for depth := 1; depth < len(relParts); depth++ {
		ancestor := relText[:ends[len(relParts)-depth]]
		if links[ancestor] {
			limit = min(limit, depth)
			break
		}
		if inPlace[ancestor] {
			limit = min(limit, depth-1)
			break
		}
	}
	staged := ""
	for depth := 0; depth <= limit; depth++ {
		key := relText[:ends[len(relParts)-depth]]
		if staged != "" {
			staged = parentPath(staged, parts[len(parts)-depth])
		}
		if _, ok := home.StagedPaths[key]; ok {
			continue
		}
		if staged == "" {
			// Walked once, for the deepest location mapped, whose prefixes
			// are the shallower ones.
			var err error
			if staged, err = load.collection.inside(parts[:len(parts)-depth]...); err != nil {
				load.collection.recordProblem(row.label + ": " + err.Error())
				return
			}
		}
		home.StagedPaths[key] = staged
	}
}

// plainParts reports whether every part passes checkElement.
func plainParts(parts []string) bool {
	for _, part := range parts {
		if checkElement(part) != nil {
			return false
		}
	}
	return true
}

// joinedPrefixes returns parts joined with slashes, and for each count k
// from 0 to len(parts) the length of the first k parts' joined text.
func joinedPrefixes(parts []string) (string, []int) {
	var joined strings.Builder
	ends := make([]int, len(parts)+1)
	for i, part := range parts {
		if i > 0 {
			joined.WriteByte('/')
		}
		joined.WriteString(part)
		ends[i+1] = joined.Len()
	}
	return joined.String(), ends
}
