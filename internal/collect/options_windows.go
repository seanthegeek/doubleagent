package collect

import (
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// absoluteProject returns PROJECT as Resolve-ProjectArgs resolves a -Project
// path, in canonical form: lexically against CWD (the native current
// directory, empty when PROJECT is absolute), whether or not it exists,
// with . and .. resolved and trailing separators trimmed. A bare drive (X:)
// is that drive's root. A path starting with one separator is rooted on
// CWD's drive. A drive-relative path (X:dir) is joined to CWD when CWD is on
// that drive, else to the drive's root, the only directory of another drive
// a new process knows.
func absoluteProject(cwd, project string) string {
	switch {
	case isBareDrive(project):
		project += `\`
	case filepath.IsAbs(project):
	case isBareDrive(filepath.VolumeName(project)):
		project = driveRelative(cwd, project)
	case strings.HasPrefix(project, `\`) || strings.HasPrefix(project, "/"):
		project = filepath.VolumeName(cwd) + project
	default:
		project = filepath.Join(cwd, project)
	}
	return syspath.FromNative(filepath.Clean(project))
}

// driveRelative resolves the drive-relative PROJECT (X:dir) against CWD
// when CWD is on drive X, else against the root of drive X.
func driveRelative(cwd, project string) string {
	drive, rest := project[:2], project[2:]
	if syspath.Equal(filepath.VolumeName(cwd), drive) {
		return filepath.Join(cwd, rest)
	}
	return drive + `\` + rest
}

// isBareDrive reports whether text is a drive letter and a colon alone.
func isBareDrive(text string) bool {
	return len(text) == 2 && text[1] == ':' &&
		(('A' <= text[0] && text[0] <= 'Z') || ('a' <= text[0] && text[0] <= 'z'))
}
