package analyze

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// absolutePath returns path made absolute against the working directory
// and cleaned, as text; "" when the working directory cannot be read.
func absolutePath(path string) string {
	if !filepath.IsAbs(path) {
		working, err := os.Getwd()
		if err != nil {
			return ""
		}
		path = filepath.Join(working, path)
	}
	return filepath.Clean(path)
}

// insideDirectory reports whether path is directory or lies below it, both
// as written, made absolute and cleaned, compared byte for byte (on Windows
// without regard to ASCII case). A spelling through a link is not seen.
func insideDirectory(path, directory string) bool {
	path, directory = syspath.Fold(absolutePath(path)), syspath.Fold(absolutePath(directory))
	if path == "" || directory == "" {
		return false
	}
	return path == directory ||
		strings.HasPrefix(path, strings.TrimSuffix(directory, string(filepath.Separator))+string(filepath.Separator))
}
