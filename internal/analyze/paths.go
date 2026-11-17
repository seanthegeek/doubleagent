package analyze

import "strings"

// pathParts returns the components of text split at separator, without
// empty and "." ones.
func pathParts(text, separator string) []string {
	var parts []string
	for _, part := range strings.Split(text, separator) {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

// pathParent returns the directory of a path printed by input.PathText, as
// Python's Path.parent gives it: "." for a name alone, the root for a name
// directly under it.
func pathParent(path string) string {
	index := strings.LastIndexAny(path, `/\`)
	switch {
	case index < 0:
		return "."
	case index == 0 || path[index-1] == ':':
		return path[:index+1]
	}
	return path[:index]
}
