//go:build windows || darwin

package stage

import "strings"

// placedAt reports whether final, the place the system gives for the file
// opened at dest (GetFinalPathNameByHandle on Windows, F_GETPATH on macOS),
// is dest's own place: the part of final below the physical staging
// directory spells the part of dest below Dir byte for byte, so a name the
// directory took for another in case or Unicode normalisation is caught.
func (stager Stager) placedAt(final, dest string) bool {
	relative, found := strings.CutPrefix(dest, stager.Dir+"/")
	if !found {
		return true
	}
	for _, root := range []string{stager.Physical, stager.Dir} {
		if rest, inside := Under(final, root); root != "" && inside {
			return rest == relative
		}
	}
	physical, err := PhysicalDir(stager.Dir)
	if rest, inside := Under(final, physical); err == nil && inside {
		return rest == relative
	}
	return true
}
