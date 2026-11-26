package analyze

import (
	"io/fs"
	"os"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// createNew creates the file at path, failing when anything is there, a
// link included, so a file is never written through an entry another
// process placed in the output directory during the run: timeline's output
// files, and the temporary file fleet writes its CSV to.
func createNew(path string, perm fs.FileMode) (*os.File, error) {
	return osfile.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
}
