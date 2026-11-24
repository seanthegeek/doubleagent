package stage

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// RunOutput names what a run writes in its output directory OutDir: the
// staging directory .stage-<RunName> and the files <RunName>.*. The zero
// RunOutput names nothing.
type RunOutput struct {
	OutDir  string
	RunName string
}

// Holds reports whether path is one of the run's outputs or lies below one,
// compared as syspath.Equal does (ignoring ASCII case on Windows). Every walk
// and the relocation sweep pass over such a path without a row or a log
// line, since the run must not read what it is writing.
func (output RunOutput) Holds(path string) bool {
	if output.RunName == "" {
		return false
	}
	rest, under := syspath.CutPrefix(path, strings.TrimSuffix(output.OutDir, "/")+"/")
	if !under {
		return false
	}
	name, _, _ := strings.Cut(rest, "/")
	return syspath.Equal(name, ".stage-"+output.RunName) ||
		syspath.HasPrefix(name, output.RunName+".")
}
