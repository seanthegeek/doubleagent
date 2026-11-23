package littlecoder

import (
	"os"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/reason"
)

// sessionIndex is one home's .pi/agent/sessions, listed once for every
// checkpoint set that looks for its pi session file there, with each of
// its project directories in name order listed once (ChildListings). A
// lookup looks only in the directories whose listing holds the name in any
// ASCII case, since a file system that matches names without regard to
// case finds the file under either spelling.
type sessionIndex struct {
	sessions string
	// failure is the problem of a sessions directory that could not be
	// looked at or listed, the answer of every lookup.
	failure  parserkit.Problems
	projects *parserkit.ChildListings
}

// sessionHeader returns the header of the pi session file name in a project
// directory of .pi/agent/sessions, null when none is found, and the
// whole-file problems of the search: the sessions directory that could not
// be listed, or, when no directory held the file (a session file name
// belongs to one project), the first project directory that could not be
// read or session file that could not be looked at, or the session file
// that could not be read. The home's sessions directory and each project
// directory are listed once per run (sessionIndex); one that could not be
// read is never looked in.
func (p *Parser) sessionHeader(artifact *input.Artifact, name string) (jsonvalue.Value, parserkit.Problems) {
	// The sessions directory is a fixed path of the home, not a neighbour
	// of the checkpoint: Home.PathOf finds it where the collection staged
	// it, at a symbolic link's physical path, which climbing from the
	// checkpoint's own directory would miss.
	if artifact.Home == nil {
		return jsonvalue.NewNull(), nil
	}
	index, held := p.sessionIndexes[artifact.Home.DiskPath]
	if !held {
		sessions, _ := artifact.Home.PathOf(".pi/agent/sessions")
		index = p.listSessions(sessions)
		p.sessionIndexes[artifact.Home.DiskPath] = index
	}
	return index.header(name, p.Work)
}

// listSessions lists the sessions directory and each of its project
// directories once.
func (p *Parser) listSessions(sessions string) *sessionIndex {
	index := &sessionIndex{sessions: sessions, projects: &parserkit.ChildListings{}}
	p.Work.Add("pi session directory listings", 1)
	var listed parserkit.Directory
	var names []string
	info, err := lstatPresent(sessions)
	if err == nil && isDirectory(info) {
		listed, _ = parserkit.DirectoryAt(sessions)
		var entries []os.DirEntry
		if entries, err = parserkit.ReadDir(sessions); err == nil {
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
		}
	}
	if err != nil {
		index.failure = parserkit.Problems{{Message: reason.Unreadable(sessions, err)}}
		return index
	}
	index.projects = parserkit.ListChildren(listed, names, lowerASCII, p.Work, "pi project directory listings")
	return index
}

// header returns the session file name from the first project directory
// in name order whose listing holds it and where it is found, else the
// first problem in that order of a directory that could not be read or
// of a holder where the file could not be looked at.
func (x *sessionIndex) header(name string, work parserkit.Work) (jsonvalue.Value, parserkit.Problems) {
	if x.failure != nil {
		return jsonvalue.NewNull(), x.failure
	}
	var firstFailure parserkit.Problems
	failedAt := -1
	for _, place := range x.projects.Holders(name) {
		work.Add("pi session file candidates", 1)
		header, failure, found := projectSession(x.projects.Directory(place).Path(), x.sessions, name)
		if found {
			return header, failure
		}
		if firstFailure == nil && failure != nil {
			firstFailure, failedAt = failure, place
		}
	}
	if failures := x.projects.Failures(); len(failures) > 0 && (firstFailure == nil || failures[0].Place < failedAt) {
		return jsonvalue.NewNull(), parserkit.Problems{failures[0].Problem()}
	}
	return jsonvalue.NewNull(), firstFailure
}

// lowerASCII returns name with its ASCII letters in lower case and every
// other byte as it is; a name is bytes, so none is decoded.
func lowerASCII(name string) string {
	folded := []byte(name)
	for i, char := range folded {
		if 'A' <= char && char <= 'Z' {
			folded[i] = char + 'a' - 'A'
		}
	}
	return string(folded)
}
