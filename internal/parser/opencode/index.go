package opencode

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
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
		sessions: map[sessionKey]jsonvalue.Value{}, projectPaths: map[projectKey]string{}}
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
// regular file that decodes, or null; it is looked up on first use.
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
		candidate, ok := childPath(projectPath, sessionID+".json")
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
