package opencode

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// treeIndex holds what the legacy tree files of one run share, each read
// once: every message file of a session looks at its directory's listing
// and at the session's file, so reading them per message file would cost
// the square of a session's length.
type treeIndex struct {
	// messages holds each message directory's sorted JSON file names.
	messages map[string][]string
	// projects holds each session directory's sorted project directories.
	projects map[string][]string
	// sessions holds each session file looked up, decoded, null when none
	// decodes.
	sessions map[sessionKey]jsonvalue.Value
	// projectPaths holds the project path each session's side files give,
	// "" when they give none.
	projectPaths map[projectKey]string
	// projectFiles holds each project file a session file read, by its
	// state, since every session of a project names it.
	projectFiles map[fileState]*sideRead
	// listings holds each project directory of a session directory, listed
	// once, since every session looks for its file in each of them.
	listings map[string]parser.Listing
}

// fileState names a file as read: its path, modification time in
// nanoseconds and size, the key of what is read from it once.
type fileState struct {
	path        string
	mtime, size int64
}

// sideRead is a side file read once: its decoded value, the error saying
// why it is not an object, and the problems reading it added.
type sideRead struct {
	value    jsonvalue.Value
	err      error
	problems parser.Problems
}

// projectKey names a session looked up under one tree root and layout.
type projectKey struct {
	root, sessionID string
	old             bool
}

// sessionKey names a session looked up under one session directory.
type sessionKey struct {
	directory, sessionID string
}

// newTreeIndex returns an empty index.
func newTreeIndex() *treeIndex {
	return &treeIndex{messages: map[string][]string{}, projects: map[string][]string{},
		sessions: map[sessionKey]jsonvalue.Value{}, projectPaths: map[projectKey]string{},
		projectFiles: map[fileState]*sideRead{}, listings: map[string]parser.Listing{}}
}

// messageNames returns the sorted JSON file names of a message directory,
// listed on first use.
func (x *treeIndex) messageNames(directory string, work parser.Work) []string {
	names, ok := x.messages[directory]
	if !ok {
		work.Add("directory listings", 1)
		if listed, found := parser.DirectoryAt(directory); found {
			names = children(listed)
		}
		x.messages[directory] = names
	}
	return names
}

// session returns the session file <project>/<id>.json under the session
// directory, from the first project directory in name order where it is a
// regular file that decodes, or null; it is looked up on first use, by its
// exact name in each project directory's listing, since the agent names
// the file by the id the message files carry.
func (x *treeIndex) session(directory, sessionID string, work parser.Work) jsonvalue.Value {
	key := sessionKey{directory: directory, sessionID: sessionID}
	if found, ok := x.sessions[key]; ok {
		return found
	}
	projects, ok := x.projects[directory]
	if !ok {
		work.Add("directory listings", 1)
		projects = subdirectories(directory)
		x.projects[directory] = projects
	}
	session := jsonvalue.NewNull()
	sessions, _ := parser.DirectoryAt(directory)
	for _, project := range projects {
		projectPath, ok := sessions.Child(project)
		if !ok {
			continue
		}
		candidate, ok := x.listing(projectPath, work).RegularFile(sessionID + ".json")
		if !ok {
			continue
		}
		if decoded, ok := quietObject(candidate, true); ok {
			session = decoded
			break
		}
	}
	x.sessions[key] = session
	return session
}

// projectFile returns the project file at path as readSide does, read once
// per state of the file for the run; the problems of that read are added
// for each lookup as a fresh read would.
func (x *treeIndex) projectFile(path string, found bool, problems *parser.Problems,
	work parser.Work) (jsonvalue.Value, error) {
	info, err := osfile.Lstat(path)
	if !found || err != nil {
		return readSide(path, found, problems)
	}
	key := fileState{path: path, mtime: info.ModTime().UnixNano(), size: info.Size()}
	held, ok := x.projectFiles[key]
	if !ok {
		work.Add("project file reads", 1)
		held = &sideRead{}
		held.value, held.err = readSide(path, true, &held.problems)
		x.projectFiles[key] = held
	}
	*problems = append(*problems, held.problems...)
	return held.value, held.err
}

// listing returns the listing of a project directory, made on first use.
func (x *treeIndex) listing(path string, work parser.Work) parser.Listing {
	listing, ok := x.listings[path]
	if !ok {
		work.Add("project directory listings", 1)
		directory, _ := parser.DirectoryAt(path)
		listing = parser.ListingOf(directory)
		x.listings[path] = listing
	}
	return listing
}
