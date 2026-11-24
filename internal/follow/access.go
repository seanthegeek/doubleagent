package follow

import "github.com/seanthegeek/doubleagent/internal/fsmeta"

// CanRead reports whether the directory path can be listed, by opening it
// through opener and, when it can be entered, reading one entry
// (fsmeta.ListError), following a link to its target; err is the root
// check's refusal or the opener's Admit's.
func CanRead(check Check, opener *fsmeta.Opener, path string) (bool, error) {
	if err := LeavesRoot(check, path); err != nil {
		return false, err
	}
	listError, refused := fsmeta.ListError(opener, path)
	return listError == nil && refused == nil, refused
}

// CanEnter reports whether the effective user may enter the directory path
// (fsmeta.EnterError), following a link to its target. A path out of the
// image root is refused.
func CanEnter(check Check, path string) (bool, error) {
	if err := LeavesRoot(check, path); err != nil {
		return false, err
	}
	return fsmeta.EnterError(path) == nil, nil
}
