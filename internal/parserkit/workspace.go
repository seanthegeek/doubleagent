package parserkit

import (
	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// ReadWorkspace returns the folder and the workspace file, as paths, of the
// workspace.json an editor keeps at file beside a workspace's state, "" for
// each when file is absent or not a regular file. One that cannot be read,
// does not decode or is not a JSON object adds a whole-file problem; the
// member names it repeats are noted in the parse of artifact. Each call
// reads the file again, so a parser that looks it up for many files keeps
// the result.
func ReadWorkspace(artifact *input.Artifact, file string, problems *Problems) (string, string) {
	if !RegularFile(file) {
		return "", ""
	}
	content, ok := FileBytes(file, problems)
	if !ok {
		return "", ""
	}
	data, err := DecodeJSON(artifact, textlines.ValidUTF8(content))
	switch {
	case err != nil:
		problems.Add(0, "workspace.json not read ("+err.Error()+")")
		return "", ""
	case !data.IsObject():
		problems.Add(0, "workspace.json is not a JSON object")
		return "", ""
	}
	return evidencepath.FileURIPath(Text(data.Get("folder"))), evidencepath.FileURIPath(Text(data.Get("workspace")))
}
