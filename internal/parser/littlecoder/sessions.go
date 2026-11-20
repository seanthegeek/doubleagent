package littlecoder

import (
	"os"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/reason"
)

// sessionIndex is one home's .pi/agent/sessions, listed once for every
// checkpoint set that looks for its pi session file there: the project
// directories in name order, and the places among them where a file name
// is to be looked at. A lookup looks at each such directory as a search of
// every directory in turn would, so the answer is the same, and at no
// other, since every other directory holds no entry of that name, so a
// search there would find nothing and fail nowhere.
type sessionIndex struct {
	sessions string
	// failure is the problem of a sessions directory that could not be
	// looked at or listed, the answer of every lookup.
	failure     parser.Problems
	directories []string
	// holders maps a name, its ASCII letters in lower case, to the places
	// of the directories whose listing holds an entry of that name in any
	// ASCII case, in name order: a file system that matches names without
	// regard to case finds the file under either spelling.
	holders map[string][]int
	// searched holds the places looked at for every name: a directory
	// whose own entry or listing could not be read, or in which a name
	// could not be looked up.
	searched []int
}

// sessionHeader returns the header of the pi session file name in a project
// directory of .pi/agent/sessions, null when none is found, and the
// whole-file problems of the search: the sessions directory that could not
// be listed, or the first project directory or session file that could not
// be looked at when no directory held the file (a session file name
// belongs to one project), or the session file that could not be read. The
// home's sessions directory is listed once per run (sessionIndex).
func (p *Parser) sessionHeader(artifact *input.Artifact, name string) (jsonvalue.Value, parser.Problems) {
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
		index = p.listSessions(sessions, name)
		p.sessionIndexes[artifact.Home.DiskPath] = index
	}
	return index.header(name, p.Work)
}

// listSessions lists the sessions directory and each of its project
// directories once; probe is a name looked up in each listed directory, to
// find those in which no name can be (a directory that can be listed but
// not searched).
func (p *Parser) listSessions(sessions, probe string) *sessionIndex {
	index := &sessionIndex{sessions: sessions, holders: map[string][]int{}}
	p.Work.Add("pi session directory listings", 1)
	var listed parser.Directory
	var names []string
	info, err := lstatPresent(sessions)
	if err == nil && isDirectory(info) {
		listed, _ = parser.DirectoryAt(sessions)
		var entries []os.DirEntry
		if entries, err = parser.ReadDir(sessions); err == nil {
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			slices.Sort(names)
		}
	}
	if err != nil {
		index.failure = parser.Problems{{Message: reason.Unreadable(sessions, err)}}
		return index
	}
	for _, name := range names {
		project, ok := listed.Child(name)
		if !ok {
			continue
		}
		index.add(project, probe, p.Work)
	}
	return index
}

// add files one project directory at the next place: under the name of
// each of its entries, or among those looked at for every name when it
// cannot be looked at, listed or searched; a directory that is not one
// (a file, a link) holds nothing.
func (x *sessionIndex) add(project, probe string, work parser.Work) {
	place := len(x.directories)
	x.directories = append(x.directories, project)
	info, err := lstatPresent(project)
	if err == nil && !isDirectory(info) {
		return
	}
	var entries []os.DirEntry
	if err == nil {
		work.Add("pi project directory listings", 1)
		entries, err = parser.ReadDir(project)
	}
	if err == nil {
		if file, named := parser.Under(project, probe); named {
			_, err = lstatPresent(file)
		}
	}
	if err != nil {
		x.searched = append(x.searched, place)
		return
	}
	for _, entry := range entries {
		key := lowerASCII(entry.Name())
		if places := x.holders[key]; len(places) == 0 || places[len(places)-1] != place {
			x.holders[key] = append(places, place)
		}
	}
}

// header returns what a search of every project directory in name order
// gives for the session file name, looking only at the directories that
// may hold it or fail for it.
func (x *sessionIndex) header(name string, work parser.Work) (jsonvalue.Value, parser.Problems) {
	if x.failure != nil {
		return jsonvalue.NewNull(), x.failure
	}
	places := x.holders[lowerASCII(name)]
	if len(x.searched) > 0 {
		places = slices.Concat(places, x.searched)
		slices.Sort(places)
	}
	var firstFailure parser.Problems
	for _, place := range places {
		work.Add("pi session file candidates", 1)
		header, failure, found := projectSession(x.directories[place], x.sessions, name)
		if found {
			return header, failure
		}
		if firstFailure == nil {
			firstFailure = failure
		}
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
