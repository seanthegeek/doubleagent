package stage

import "os"

// occupantOf returns the staged path of the entry the copy open on file
// sits at, and true, when that is not dest: Linux gives no exact path for a
// descriptor, so each component below the staging directory is looked up
// in its directory's listing (occupantBelow).
func (stager Stager) occupantOf(_ *os.File, dest string) (string, bool) {
	return occupantBelow(stager.Dir, dest)
}
