package opencode

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// treeIndex holds what the legacy tree files of one home share, each read
// once: every message file of a session looks at its directory's listing
// and at the session's file, so reading them per message file would cost
// the square of a session's length.
type treeIndex struct {
	// messages holds each message directory's sorted JSON file names.
	messages map[string][]string
	// projects holds each session directory's project directories, listed
	// once, since every session looks for its file in each of them.
	projects map[string]*parserkit.ChildListings
	// sessions holds each session file looked up, decoded, null when none
	// decodes.
	sessions map[sessionKey]jsonvalue.Value
	// projectPaths holds the project path each session's side files give,
	// "" when they give none.
	projectPaths map[projectKey]string
	// projectFiles holds each project file a session file read, by its
	// state, since every session of a project names it.
	projectFiles map[fileState]*sideRead
	// quietSides holds each side file a session's scope read quietly, by
	// its state, null when it is not an object, since sessions naming one
	// file in several spellings the disk opens alike would each read it.
	quietSides map[fileState]jsonvalue.Value
	// failedLooks holds each side file path whose look failed, since every
	// session of a project names its file and a file not there stays so
	// while its home is read.
	failedLooks map[string]bool
	// partsShown holds the message file that first showed each part
	// directory, by the directory's entry, since a message id names one
	// directory of parts.
	partsShown map[parserkit.EntryKey]shownBy
}

// shownBy is the message file that showed a part directory: its path on
// disk, which tells it from a later file, and its source path, which a
// later file's problem row names.
type shownBy struct {
	disk, original string
}

// fileState names a file as read: its entry, modification time in
// nanoseconds and size, the key of what is read from it once.
type fileState struct {
	entry       parserkit.EntryKey
	mtime, size int64
}

// sideRead is a side file read once: its decoded value, the error saying
// why it is not an object, and the problems reading it added.
type sideRead struct {
	value    jsonvalue.Value
	err      error
	problems parserkit.Problems
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
	return &treeIndex{messages: map[string][]string{}, projects: map[string]*parserkit.ChildListings{},
		sessions: map[sessionKey]jsonvalue.Value{}, projectPaths: map[projectKey]string{},
		projectFiles: map[fileState]*sideRead{}, quietSides: map[fileState]jsonvalue.Value{},
		failedLooks: map[string]bool{}, partsShown: map[parserkit.EntryKey]shownBy{}}
}

// messageNames returns the sorted JSON file names of a message directory,
// listed on first use.
func (x *treeIndex) messageNames(artifact *input.Artifact, directory string, work parserkit.Work) []string {
	names, ok := x.messages[directory]
	if !ok {
		work.Add("directory listings", 1)
		if listed, found := parserkit.DirectoryAt(artifact, directory); found {
			names = children(listed)
		}
		x.messages[directory] = names
	}
	return names
}

// session returns the session file <project>/<id>.json under the session
// directory, from the first project directory in name order where it is a
// regular file that decodes, or null; it is looked up on first use, by its
// exact name in the project directories' listings, since the agent names
// the file by the id the message files carry. A project directory that
// could not be listed is not looked in: the lookups stay quiet, as the
// session file's own problems are reported where it is parsed.
func (x *treeIndex) session(artifact *input.Artifact, directory, sessionID string, work parserkit.Work,
) jsonvalue.Value {
	key := sessionKey{directory: directory, sessionID: sessionID}
	if found, ok := x.sessions[key]; ok {
		return found
	}
	files, ok := x.projects[directory]
	if !ok {
		work.Add("directory listings", 1)
		files = listProjects(artifact, directory, work)
		x.projects[directory] = files
	}
	session := jsonvalue.NewNull()
	name := sessionID + ".json"
	for _, place := range files.Holders(name) {
		work.Add("session file candidates", 1)
		candidate, ok := files.Directory(place).Child(name)
		if !ok || !parserkit.RegularFile(candidate) {
			continue
		}
		if decoded, ok := quietObject(nil, candidate, true); ok {
			session = decoded
			break
		}
	}
	x.sessions[key] = session
	return session
}

// listProjects lists the project directories of a session directory and
// each of them once; one that could not be read holds no session file.
// A name the session directory does not give as a plain component is
// passed over.
func listProjects(artifact *input.Artifact, directory string, work parserkit.Work) *parserkit.ChildListings {
	sessions, _ := parserkit.DirectoryAt(artifact, directory)
	return parserkit.ListChildren(sessions, subdirectories(directory), nil, work, "project directory listings")
}

// projectFile returns the project file at path as readSide does, read once
// per state of the file for the home, whatever spelling of its name opens
// it, and looked for once when it is not there (sideState); the problems
// of that read are added for each lookup as a fresh read would.
func (x *treeIndex) projectFile(path string, found bool, problems *parserkit.Problems,
	work parserkit.Work) (jsonvalue.Value, error) {
	key, looked := x.sideState(path, found, work)
	if !looked {
		return jsonvalue.NewNull(), nil
	}
	held, ok := x.projectFiles[key]
	if !ok {
		work.Add("project file reads", 1)
		held = &sideRead{}
		held.value, held.err = readSide(nil, path, true, &held.problems)
		x.projectFiles[key] = held
	}
	*problems = append(*problems, held.problems...)
	return held.value, held.err
}

// quietSide returns the side file at path decoded as an object as
// quietObject does, null when it is not one, read once per state of the
// file for the home, whatever spelling of its name opens it, and looked
// for once when it is not there (sideState).
func (x *treeIndex) quietSide(path string, found bool, work parserkit.Work) jsonvalue.Value {
	key, looked := x.sideState(path, found, work)
	if !looked {
		return jsonvalue.NewNull()
	}
	value, ok := x.quietSides[key]
	if !ok {
		work.Add("quiet side reads", 1)
		value, _ = quietObject(nil, path, true)
		x.quietSides[key] = value
	}
	return value
}

// sideState returns the state of the side file at path, the key of what is
// read from it once, or false when the path cannot be reached or looked
// at: such a file is neither a regular file nor readable, so it gives null.
// A look that fails is remembered by the path, so the file is looked for
// once for the home however many sessions name it.
func (x *treeIndex) sideState(path string, found bool, work parserkit.Work) (fileState, bool) {
	if !found || x.failedLooks[path] {
		return fileState{}, false
	}
	info, err := osfile.Lstat(path)
	if err != nil {
		work.Add("failed side looks", 1)
		x.failedLooks[path] = true
		return fileState{}, false
	}
	return fileState{entry: parserkit.KeyOf(path), mtime: info.ModTime().UnixNano(), size: info.Size()}, true
}
