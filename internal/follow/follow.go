// Package follow is the one place the binary follows a link in the last
// component of a path derived from the image root: a stat, a directory or
// regular-file test, an access check, a directory listing test, a read of a
// whole file or a resolution to the physical directory. Each function takes
// the run's root check and asks it about the full path before following
// anything; when the path leads out of the image root it follows nothing
// and returns an *OutOfRootError naming the link, which the caller handles.
// A live run passes a nil check, and the functions follow as the operating
// system does.
//
// A test in this package denies by default. Outside the directories of this
// package and the ones it is built on (osfile, fsmeta, stage), every non-test
// Go file of the module may import only a reviewed standard or vendored
// package, and may use an exported name of a package that can act on a file
// path (os, os/exec, os/user, path/filepath, syscall, x/sys/unix,
// x/sys/windows and its registry, osfile, fsmeta, stage) only when a list
// says it takes no path, or never follows its last component and changes
// nothing, or a counted list gives the call's reason. The test reads
// package-level names only, so it cannot see a method or a call through an
// interface or a function value; the methods in use take a path and do not
// follow its last component (fsmeta.Opener.Open and OpenRegular open with
// O_NOFOLLOW or FILE_FLAG_OPEN_REPARSE_POINT; stage.Stager.CopyFile reads
// its source through them; hostcmd.Plutil runs plutil on a hint file whose
// own lstat typed it a regular file).
package follow

import (
	"errors"
	"io/fs"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/stage"
)

// Kind says why the root check refused a path.
type Kind int

// The kinds of refusal. Each log line that reports one words it by its
// kind (Reason), so a refusal is never described as what it is not.
const (
	// LeadsOut is a link whose target leads out of the image root.
	LeadsOut Kind = iota + 1
	// Unverified is a link the check could not follow (a loop, more than
	// maxLinkHops links, a target it cannot read or place) or a root it
	// could not resolve.
	Unverified
	// OtherReparse is a reparse point other than a symlink or junction.
	OtherReparse
	// Climbs is a path whose own `..` climbs above the image root.
	Climbs
	// Outside is a path not under the image root as given.
	Outside
	// OpenedOutside is a path the check let through whose open handle
	// Windows places outside the image root (Confinement.Admit): a mistake
	// of the check, caught after the open and before anything is read.
	OpenedOutside
)

// Refusal is the root check's decision against a path: the path that
// decides it (the link, or the path itself for Climbs and Outside) and why.
type Refusal struct {
	Link string
	Kind Kind
}

// Check reports whether the image root's check refuses path, and why;
// walk.RootCheck provides it, nil on a live run.
type Check func(path string) (Refusal, bool)

// OutOfRootError says that the root check refused Path, so nothing was
// followed.
type OutOfRootError struct {
	Path string
	Refusal
}

// Error describes the refusal.
func (e *OutOfRootError) Error() string {
	return e.Path + ":" + Reason(e.Refusal, " leads out of the image root at "+e.Link)
}

// LeavesRoot returns the error that says the check refuses path, or nil
// when check is nil or lets it through.
func LeavesRoot(check Check, path string) error {
	if check == nil {
		return nil
	}
	if refusal, refused := check(path); refused {
		return &OutOfRootError{Path: path, Refusal: refusal}
	}
	return nil
}

// OutOfRoot returns the refusal err carries when it is an *OutOfRootError.
func OutOfRoot(err error) (Refusal, bool) {
	var refused *OutOfRootError
	if errors.As(err, &refused) {
		return refused.Refusal, true
	}
	return Refusal{}, false
}

// Reason returns the end of a log line, after the refused path, for
// refusal: leadsOut, the line's own words starting with their separator,
// when a link leads out of the root, and the words of its kind otherwise,
// after a space.
func Reason(refusal Refusal, leadsOut string) string {
	const nothing = "; nothing behind it was looked at"
	switch refusal.Kind {
	case Unverified:
		return " cannot be verified to lie inside the image root" + nothing
	case OtherReparse:
		return " is a reparse point other than a symlink or junction" + nothing
	case Climbs:
		return " climbs above the image root with .." + nothing
	case Outside:
		return " is outside the image root" + nothing
	case OpenedOutside:
		return ": the root check allowed it but Windows opened it outside the image root;" +
			" nothing from it was used"
	}
	return leadsOut
}

// Stat is os.Stat of path, the last component followed.
func Stat(check Check, path string) (fs.FileInfo, error) {
	if err := LeavesRoot(check, path); err != nil {
		return nil, err
	}
	return osfile.Stat(path)
}

// IsDirectory is test -d: whether path is a directory or a link to one.
func IsDirectory(check Check, path string) (bool, error) {
	info, err := Stat(check, path)
	if _, out := OutOfRoot(err); out {
		return false, err
	}
	return err == nil && info.IsDir(), nil
}

// IsRegularFile is test -f: whether path is a regular file or a link to one.
func IsRegularFile(check Check, path string) (bool, error) {
	info, err := Stat(check, path)
	if _, out := OutOfRoot(err); out {
		return false, err
	}
	return err == nil && info.Mode().IsRegular(), nil
}

// ReadFile returns the bytes of the file path names, the last component
// followed, as test -f and cat follow it.
func ReadFile(check Check, path string) ([]byte, error) {
	if err := LeavesRoot(check, path); err != nil {
		return nil, err
	}
	return osfile.ReadFile(path)
}

// DirReadError is fsmeta.DirReadError of path: why the directory it names
// cannot be listed or entered. A path that leads out of the image root is
// reported as failed, with the refusal as its reason and in err, so a caller
// that drops err cannot read it as readable.
func DirReadError(check Check, path string) (reason string, failed bool, err error) {
	if err := LeavesRoot(check, path); err != nil {
		return err.Error(), true, err
	}
	reason, failed = fsmeta.DirReadError(path)
	return reason, failed, nil
}

// PhysicalDir is stage.PhysicalDir of path: the directory it resolves to,
// every link followed.
func PhysicalDir(check Check, path string) (string, error) {
	if err := LeavesRoot(check, path); err != nil {
		return "", err
	}
	return stage.PhysicalDir(path)
}
