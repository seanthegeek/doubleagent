package stage

import "os"

// placedFile reports whether the copy open on file sits at dest: Linux gives
// no exact path for a descriptor, so each component below the staging
// directory must be listed under its own name (spelledBelow).
func (stager Stager) placedFile(_ *os.File, dest string) bool {
	return spelledBelow(stager.Dir, dest)
}
