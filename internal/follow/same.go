package follow

import (
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// SameEntry returns the run's answer to whether two spellings of a path
// whose syspath.Fold keys are equal name one file system entry: both pass
// the root check and fsmeta.SameEntry says so (one non-zero identity, or
// neither exists). A path the check refuses is not looked at, and
// the two stay apart.
func SameEntry(check Check) syspath.SameEntry {
	return func(first, second string) bool {
		return LeavesRoot(check, first) == nil && LeavesRoot(check, second) == nil &&
			fsmeta.SameEntry(first, second)
	}
}
