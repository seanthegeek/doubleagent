package opencode

import (
	"path/filepath"

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
	// listings counts the directories listed, the work a test measures.
	listings int
}

// sessionKey names a session looked up under one session directory.
type sessionKey struct {
	directory, sessionID string
}

// newTreeIndex returns an empty index.
func newTreeIndex() *treeIndex {
	return &treeIndex{messages: map[string][]string{}, projects: map[string][]string{},
		sessions: map[sessionKey]jsonvalue.Value{}}
}

// messageNames returns the sorted JSON file names of a message directory,
// listed on first use.
func (x *treeIndex) messageNames(directory string) []string {
	names, ok := x.messages[directory]
	if !ok {
		x.listings++
		names = children(directory)
		x.messages[directory] = names
	}
	return names
}

// session returns the session file <project>/<id>.json under the session
// directory, from the first project directory in name order where it is a
// regular file that decodes, or null; it is looked up on first use.
func (x *treeIndex) session(directory, sessionID string) jsonvalue.Value {
	key := sessionKey{directory: directory, sessionID: sessionID}
	if found, ok := x.sessions[key]; ok {
		return found
	}
	projects, ok := x.projects[directory]
	if !ok {
		x.listings++
		projects = subdirectories(directory)
		x.projects[directory] = projects
	}
	session := jsonvalue.NewNull()
	for _, project := range projects {
		candidate, ok := childPath(filepath.Join(directory, project), sessionID+".json")
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
