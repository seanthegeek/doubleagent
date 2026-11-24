package stage

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// RunOutput names what a run writes by the run's name, RunName: the staging
// directory .stage-<RunName> and the files <RunName>.*. The zero RunOutput
// names nothing.
type RunOutput struct {
	RunName string
}

// Holds reports whether a component of path is .stage-<RunName> or starts
// with <RunName>., compared as syspath.Equal does (ignoring ASCII case on
// Windows). The name holds the host and the run's start time, so it is the
// run's alone: a file by that name is the run's own or a copy of it, and no
// spelling of the output directory (a link, another letter case, /tmp for
// /private/tmp) hides one from the test. The walk's row writers write no
// row of such a path, and the walks, project discovery and the relocation
// search pass over it, since the run must not read what it is writing.
func (output RunOutput) Holds(path string) bool {
	if output.RunName == "" {
		return false
	}
	stageName, filePrefix := ".stage-"+output.RunName, output.RunName+"."
	for component := range strings.SplitSeq(path, "/") {
		if syspath.Equal(component, stageName) || syspath.HasPrefix(component, filePrefix) {
			return true
		}
	}
	return false
}
