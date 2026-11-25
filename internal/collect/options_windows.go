package collect

import (
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// absoluteProject returns PROJECT, a -p path, in canonical form, resolved by
// name against CWD (the native current directory, empty when PROJECT is
// absolute) with . and .. removed and trailing separators trimmed. A bare
// drive (X:) is that drive's root; a path starting with one separator is
// rooted on CWD's drive; a drive-relative path (X:dir) is joined to CWD when
// CWD is on that drive, else to the drive's root, the only directory of
// another drive a new process knows. Nothing is looked up on disk.
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

// absoluteRoot returns ROOT in canonical form, resolved by name against CWD
// as a -p path is, and its native form as the path tested as a directory,
// since a bare drive's root is X:\ to Win32, never X:. Existing components
// take the disk's spelling from the drive down (a missing one stays as
// typed), since -r is a path on the analyst's own host.
func absoluteRoot(cwd, root string) (resolved, checked string) {
	resolved = fsmeta.OnDiskPath("", absoluteProject(cwd, root))
	return resolved, syspath.ToNative(resolved)
}

// typedRoot returns ROOT made absolute against CWD as typed, before
// absoluteRoot respells it from the disk, so a -p spelled like -r can be
// matched with it.
func typedRoot(cwd, root, _ string) string { return absoluteProject(cwd, root) }
