package parser

import (
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// Beside returns the path of names below the directory that holds the
// artifact's file: where a parser looks for the side files an agent keeps
// next to a transcript or a database. It is the one way a parser reaches a
// neighbouring file; the path is text, and reading it goes through the
// kit's readers, which never follow a link.
func Beside(artifact *input.Artifact, names ...string) string {
	return filepath.Join(append([]string{filepath.Dir(artifact.DiskPath)}, names...)...)
}

// Above returns the path of names below the directory levels above the one
// that holds the artifact's file (0 is that directory itself), and false
// when that would climb past the artifact's home: a file lying depth
// directories below its home can go up at most depth levels, so no parser
// walks out of the home it was given, whatever the names in the evidence.
func Above(artifact *input.Artifact, levels int, names ...string) (string, bool) {
	if levels < 0 || levels > strings.Count(artifact.Rel, "/") {
		return "", false
	}
	directory := filepath.Dir(artifact.DiskPath)
	for range levels {
		directory = filepath.Dir(directory)
	}
	return filepath.Join(append([]string{directory}, names...)...), true
}
