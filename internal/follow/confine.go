package follow

import (
	"strings"
	"sync"
)

// Confinement is a second check, independent of the root check's walk: after
// the binary opens a path the root check let through, it asks the open
// handle where it is and refuses the open unless that lies under the image
// root's own place, so a mistake of the walk shows as a log line instead of
// as the analyst's own files in the archive. It is built only for an image
// run on Windows (NewConfinement); a nil Confinement admits everything.
type Confinement struct {
	root   string
	where  func(handle uintptr) (string, error)
	report func(path string)
	mutex  sync.Mutex
	seen   map[string]bool
}

// newConfinement returns the confinement under root, the place where reports
// for the image root, with where asking a handle for its place and report
// receiving each refused path once.
func newConfinement(root string, where func(handle uintptr) (string, error),
	report func(path string),
) *Confinement {
	return &Confinement{root: strings.TrimSuffix(root, `\`), where: where, report: report,
		seen: map[string]bool{}}
}

// liesUnder reports whether place is root or below it. Both come from the
// same system call, which spells every component as the disk holds it, so
// they compare byte for byte: folding case would let in a sibling that a
// case-sensitive directory keeps apart from the root.
func liesUnder(place, root string) bool {
	return place == root || strings.HasPrefix(place, root+`\`)
}

// Admit returns nil when handle, just opened on path and used for nothing
// else yet (os.Open, behind fsmeta.OpenDir, has only asked GetFileType of
// it), lies under the image root, and otherwise an *OutOfRootError of kind
// OpenedOutside, reporting path the first time. A handle whose place cannot
// be asked is refused too. The caller closes a refused handle and uses
// nothing from it.
func (confinement *Confinement) Admit(path string, handle uintptr) error {
	if confinement == nil {
		return nil
	}
	if place, err := confinement.where(handle); err == nil && liesUnder(place, confinement.root) {
		return nil
	}
	confinement.mutex.Lock()
	first := !confinement.seen[path]
	confinement.seen[path] = true
	confinement.mutex.Unlock()
	if first && confinement.report != nil {
		confinement.report(path)
	}
	return &OutOfRootError{Path: path, Refusal: Refusal{Link: path, Kind: OpenedOutside}}
}
