package input

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// relativeToHome returns path relative to home, both host paths with
// slashes or backslashes, as a slash path without a leading slash; a path
// outside home is returned whole, less its leading slashes.
func relativeToHome(path, home string) string {
	slashPath := strings.ReplaceAll(path, `\`, "/")
	slashHome := strings.TrimRight(strings.ReplaceAll(home, `\`, "/"), "/")
	if slashHome != "" && (slashPath == slashHome || strings.HasPrefix(slashPath, slashHome+"/")) {
		return strings.TrimLeft(slashPath[len(slashHome):], "/")
	}
	return strings.TrimLeft(slashPath, "/")
}

// archiveDirFromRow returns `fs/<home>` as the collector laid the home out,
// derived from one row, or "" for a row archived at a symlink's physical
// path: the archive path must end with the home-relative path, and what is
// left must be the home's drive form or fs plus the home less a leading
// root, never the same path below another directory.
func archiveDirFromRow(row jsonvalue.Value) string {
	home := textOf(row.Get("home"))
	archivePath := strings.ReplaceAll(textOf(row.Get("archive_path")), `\`, "/")
	rel := relativeToHome(textOf(row.Get("path")), home)
	if rel == "" || !strings.HasSuffix(archivePath, "/"+rel) {
		return ""
	}
	archiveDir := archivePath[:len(archivePath)-len(rel)-1]
	if archiveDir == archiveDirFallback(home, "") {
		return archiveDir
	}
	slashHome := strings.ReplaceAll(home, `\`, "/")
	if strings.HasPrefix(archiveDir, "fs/") && strings.HasSuffix(slashHome, archiveDir[len("fs"):]) {
		return archiveDir
	}
	return ""
}

// archiveDirFallback returns `fs/<home>` as the collector lays a home out
// when no row has shown it: less the image root of an image run, and a
// Windows path as `fs/<drive letter>/<path>` (C:\Users\bob is
// fs/C/Users/bob).
func archiveDirFallback(home, imageRoot string) string {
	slashHome := strings.ReplaceAll(home, `\`, "/")
	slashRoot := strings.TrimRight(strings.ReplaceAll(imageRoot, `\`, "/"), "/")
	if slashRoot != "" && strings.HasPrefix(slashHome, slashRoot+"/") {
		return "fs/" + slashHome[len(slashRoot)+1:]
	}
	if windowsDrive(slashHome) != "" {
		return "fs/" + strings.ToUpper(slashHome[:1]) + "/" + strings.TrimLeft(slashHome[2:], "/")
	}
	return "fs" + slashHome
}

// archiveParts splits an archive path at slashes or backslashes, dropping
// empty parts.
func archiveParts(archivePath string) []string {
	return splitNonEmpty(strings.ReplaceAll(archivePath, `\`, "/"))
}

// splitNonEmpty splits a slash path, dropping empty parts.
func splitNonEmpty(path string) []string {
	var parts []string
	for _, part := range splitSlash(path) {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// archivedInPlace reports whether archivePath is where the home's own
// directory holds rel.
func archivedInPlace(home *Home, root, rel, archivePath string) bool {
	return samePath(joinPath(root, archiveParts(archivePath)...),
		joinPath(home.DiskPath, splitNonEmpty(rel)...))
}

// mapStagedPath records in the home's staged paths a row archived away from
// DiskPath/rel: the file and each directory above it, down to the symlinked
// directory that moved it, map to their place in the archive. That
// directory is the deepest ancestor of rel among links; without one (an
// excluded link has no row) the components are matched from the right
// while equal, and the walk stops below a directory in inPlace. A location
// inside refuses is a problem of the row and maps nothing.
func (load *manifestLoad) mapStagedPath(home *Home, row archivedRow, links, inPlace map[string]bool) {
	relParts := splitNonEmpty(row.rel)
	parts := archiveParts(row.archivePath)
	if len(relParts) == 0 || archivedInPlace(home, load.collection.Root, row.rel, row.archivePath) {
		return
	}
	if startsOutside(row.archivePath) {
		load.collection.recordProblem(row.label + ": " + errOutside.Error())
		return
	}
	equal := 0
	for equal < len(relParts) && equal < len(parts) &&
		relParts[len(relParts)-1-equal] == parts[len(parts)-1-equal] {
		equal++
	}
	limit := min(equal, len(relParts)-1)
	for depth := 1; depth < len(relParts); depth++ {
		ancestor := joinSlash(relParts[:len(relParts)-depth])
		if links[ancestor] {
			limit = min(limit, depth)
			break
		}
		if inPlace[ancestor] {
			limit = min(limit, depth-1)
			break
		}
	}
	for depth := 0; depth <= limit; depth++ {
		key := joinSlash(relParts[:len(relParts)-depth])
		if _, ok := home.StagedPaths[key]; ok {
			continue
		}
		staged, err := load.collection.inside(parts[:len(parts)-depth]...)
		if err != nil {
			load.collection.recordProblem(row.label + ": " + err.Error())
			return
		}
		home.StagedPaths[key] = staged
	}
}
