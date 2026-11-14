package relocation

import "github.com/seanthegeek/doubleagent/internal/stage"

// physicalDirectory returns the physical path of directory in canonical
// form, every symlink and junction resolved, as Get-LinkedParent resolves a
// link; ok is false when it is no directory or cannot be opened. Windows has
// no search permission bit to test.
func physicalDirectory(directory string) (physical string, ok bool) {
	physical, err := stage.PhysicalDir(directory)
	return physical, err == nil
}
