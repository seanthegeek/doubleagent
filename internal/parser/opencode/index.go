package opencode

import (
	"slices"

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
	// projects holds each session directory's project directories, listed
	// once, since every session looks for its file in each of them.
	projects map[string]*projectFiles
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
	// partsShown holds the message file that first showed each part
	// directory, by the directory's entry, since a message id names one
	// directory of parts.
	partsShown map[parser.EntryKey]string
}

// projectFiles is one session directory's project directories, in name
// order, each listed once, with the places of those holding each file
// name as a regular file, so finding a session's file costs the number of
// directories holding its name rather than the number of directories.
type projectFiles struct {
	directories []parser.Directory
	// holders maps a file name to the places in directories of the listed
	// directories that hold it as a regular file, in name order.
	holders map[string][]int
	// unlisted holds the places of the directories that could not be
	// listed, where every name is looked at.
	unlisted []int
}

// fileState names a file as read: its entry, modification time in
// nanoseconds and size, the key of what is read from it once.
type fileState struct {
	entry       parser.EntryKey
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
	return &treeIndex{messages: map[string][]string{}, projects: map[string]*projectFiles{},
		sessions: map[sessionKey]jsonvalue.Value{}, projectPaths: map[projectKey]string{},
		projectFiles: map[fileState]*sideRead{}, quietSides: map[fileState]jsonvalue.Value{}, partsShown: map[parser.EntryKey]string{}}
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
// exact name in the project directories' listings, since the agent names
// the file by the id the message files carry.
func (x *treeIndex) session(directory, sessionID string, work parser.Work) jsonvalue.Value {
	key := sessionKey{directory: directory, sessionID: sessionID}
	if found, ok := x.sessions[key]; ok {
		return found
	}
	files, ok := x.projects[directory]
	if !ok {
		work.Add("directory listings", 1)
		files = listProjects(directory, work)
		x.projects[directory] = files
	}
	session := jsonvalue.NewNull()
	name := sessionID + ".json"
	for _, place := range files.candidates(name) {
		work.Add("session file candidates", 1)
		candidate, ok := files.directories[place].Child(name)
		if !ok || !parser.RegularFile(candidate) {
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

// listProjects lists the project directories of a session directory and
// each of them once. A name the session directory does not give as a plain
// component is passed over.
func listProjects(directory string, work parser.Work) *projectFiles {
	files := &projectFiles{holders: map[string][]int{}}
	sessions, _ := parser.DirectoryAt(directory)
	for _, project := range subdirectories(directory) {
		projectPath, ok := sessions.Child(project)
		if !ok {
			continue
		}
		work.Add("project directory listings", 1)
		listed, _ := parser.DirectoryAt(projectPath)
		place := len(files.directories)
		files.directories = append(files.directories, listed)
		entries, err := parser.ReadDir(listed.Path())
		if err != nil {
			files.unlisted = append(files.unlisted, place)
			continue
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				files.holders[entry.Name()] = append(files.holders[entry.Name()], place)
			}
		}
	}
	return files
}

// candidates returns the places of the directories where name may be a
// regular file, in name order: those whose listing holds it as one and
// those that could not be listed.
func (f *projectFiles) candidates(name string) []int {
	held := f.holders[name]
	if len(f.unlisted) == 0 {
		return held
	}
	merged := slices.Concat(held, f.unlisted)
	slices.Sort(merged)
	return merged
}

// projectFile returns the project file at path as readSide does, read once
// per state of the file for the run, whatever spelling of its name opens
// it; the problems of that read are added for each lookup as a fresh read
// would.
func (x *treeIndex) projectFile(path string, found bool, problems *parser.Problems,
	work parser.Work) (jsonvalue.Value, error) {
	info, err := osfile.Lstat(path)
	if !found || err != nil {
		return readSide(path, found, problems)
	}
	key := fileState{entry: parser.KeyOf(path), mtime: info.ModTime().UnixNano(), size: info.Size()}
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

// quietSide returns the side file at path decoded as an object as
// quietObject does, null when it is not one, read once per state of the
// file for the run, whatever spelling of its name opens it.
func (x *treeIndex) quietSide(path string, found bool, work parser.Work) jsonvalue.Value {
	info, err := osfile.Lstat(path)
	if !found || err != nil {
		value, _ := quietObject(path, found)
		return value
	}
	key := fileState{entry: parser.KeyOf(path), mtime: info.ModTime().UnixNano(), size: info.Size()}
	value, ok := x.quietSides[key]
	if !ok {
		work.Add("quiet side reads", 1)
		value, _ = quietObject(path, true)
		x.quietSides[key] = value
	}
	return value
}
