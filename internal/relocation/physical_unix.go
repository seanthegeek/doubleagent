//go:build unix

package relocation

import "github.com/seanthegeek/doubleagent/internal/follow"

// physicalDirectory returns the physical path of directory ("" for the
// root), every link resolved, as `cd -P DIR && pwd -P` prints it; ok is false
// where cd would fail (no such directory, or no search permission on it) and
// for a directory check says leads out of the image root, which is not
// followed.
func physicalDirectory(check follow.Check, directory string) (physical string, ok bool) {
	if directory == "" {
		directory = "/"
	}
	physical, err := follow.PhysicalDir(check, directory)
	if err != nil {
		return "", false
	}
	isDir, _ := follow.IsDirectory(check, physical)
	canEnter, _ := follow.CanEnter(check, physical)
	return physical, isDir && canEnter
}
