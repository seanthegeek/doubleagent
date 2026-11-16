//go:build darwin || linux

package stage

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// occupantBelow returns path with each component below base spelled as its
// directory lists the entry it resolves to, and true, when that differs
// from path: a directory that folds case resolved a component to an entry
// spelled otherwise. A component listed only under another name than case
// tells apart (Unicode normalisation) is left as it is, and still counts as
// differing.
func occupantBelow(base, path string) (string, bool) {
	rest, found := strings.CutPrefix(path, base+"/")
	if !found {
		return "", false
	}
	directory, differs := base, false
	for _, component := range strings.Split(rest, "/") {
		entries, err := osfile.ReadDir(directory)
		if err != nil {
			return directory, true
		}
		spelled, exact := "", false
		for _, entry := range entries {
			if entry.Name() == component {
				spelled, exact = component, true
				break
			}
			if spelled == "" && strings.EqualFold(entry.Name(), component) {
				spelled = entry.Name()
			}
		}
		if !exact {
			differs = true
			if spelled == "" {
				spelled = component
			}
		}
		directory += "/" + spelled
	}
	return directory, differs
}
