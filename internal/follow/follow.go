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

// DirReadError is fsmeta.DirReadError of path: why the directory it names
// cannot be listed or entered. A path that leads out of the image root is
// reported as failed, with the refusal as its reason and in err, so a caller
// that drops err cannot read it as readable. The listing opens through
// opener; one Windows opened outside the image root is failed with no
// reason and the OpenedOutside refusal in err, and its caller writes no row.
func DirReadError(check Check, opener *fsmeta.Opener, path string,
) (reason string, failed bool, err error) {
	if err := LeavesRoot(check, path); err != nil {
		return err.Error(), true, err
	}
	reason, failed, err = fsmeta.DirReadError(opener, path)
	return reason, failed, err
}

// IsOpenedOutside reports whether err is the refusal of a path Windows opened
// outside the image root (Confinement.Admit), already logged once.
func IsOpenedOutside(err error) bool {
	refusal, refused := OutOfRoot(err)
	return refused && refusal.Kind == OpenedOutside
}

// PhysicalDir is stage.PhysicalDir of path: the directory it resolves to,
// every link followed.
func PhysicalDir(check Check, path string) (string, error) {
	if err := LeavesRoot(check, path); err != nil {
		return "", err
	}
	return stage.PhysicalDir(path)
}
