package relocation

import "github.com/seanthegeek/doubleagent/internal/follow"

// physicalDirectory returns the physical path of directory in canonical
// form, every symlink and junction resolved, as Get-LinkedParent resolves a
// link; ok is false when it is no directory, cannot be opened, or leads out
// of the image root by check, which is not followed. Windows has no search
// permission bit to test.
func physicalDirectory(check follow.Check, directory string) (physical string, ok bool) {
	physical, err := follow.PhysicalDir(check, directory)
	return physical, err == nil
}
