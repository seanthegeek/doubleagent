//go:build windows || darwin

package stage

import "strings"

// occupantAt returns, for final, the place the system gives for the file
// opened at dest (GetFinalPathNameByHandle on Windows, F_GETPATH on macOS),
// the staged path of the entry that is there, spelled below Dir, and true,
// when it is not dest's own place: the part of final below the physical
// staging directory is compared with the part of dest below Dir byte for
// byte, so a name the directory took for another in case or Unicode
// normalisation is caught. A place outside the staging directory, or one it
// cannot place, proves nothing and gives false.
func (stager Stager) occupantAt(final, dest string) (string, bool) {
	relative, found := strings.CutPrefix(dest, stager.Dir+"/")
	if !found {
		return "", false
	}
	roots := []string{stager.Physical, stager.Dir}
	if physical, err := PhysicalDir(stager.Dir); err == nil {
		roots = append(roots, physical)
	}
	for _, root := range roots {
		if rest, inside := Under(final, root); root != "" && inside {
			return stager.Dir + "/" + rest, rest != relative
		}
	}
	return "", false
}
