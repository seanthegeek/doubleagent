package analyze

import "strings"

// pathParent returns the directory part of a host path, as filepath.Dir
// gives it for a cleaned path: "." for a name alone, the root for a name
// directly under it. It stands in for filepath.Dir, which the import guard
// allows in no file of this package.
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
