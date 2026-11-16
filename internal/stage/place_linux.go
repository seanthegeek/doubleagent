package stage

import "os"

// occupantOf returns the staged path of the entry the copy open on file
// sits at, and true, when that is not dest: Linux gives no exact path for a
// descriptor, so the staged path is walked as the tree spells it
// (walkStaged); a walk that does not reach the end proves nothing.
func (stager Stager) occupantOf(_ *os.File, dest string) (string, bool) {
	spelling := stager.spelling(dest)
	if spelling.stop != stagedEnd || !spelling.respelled {
		return "", false
	}
	return spelling.path, true
}
