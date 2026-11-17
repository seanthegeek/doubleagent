package opencode

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// treeRoot is the directory a legacy tree file's paths are relative to:
// its original path on the source host with the separator that path uses,
// and where the collection holds it.
type treeRoot struct {
	original  string
	separator string
	disk      string
}

// rootOf returns the directory depth levels above the artifact (the
// artifact's own directory is the first), and false when that lies outside
// the artifact's home. The original path is cut by the length of the
// relative tail, which is the same whatever the separator.
func rootOf(artifact *input.Artifact, depth int) (treeRoot, bool) {
	disk, found := parser.Above(artifact, depth-1)
	if !found {
		return treeRoot{}, false
	}
	parts := strings.Split(artifact.Rel, "/")
	depth = min(depth, len(parts))
	tail := strings.Join(parts[len(parts)-depth:], "/")
	original := artifact.Original
	separator := "/"
	if strings.Count(original, `\`) > strings.Count(original, "/") {
		separator = `\`
	}
	cut := original[:max(len(original)-len(tail), 0)]
	return treeRoot{original: strings.TrimRight(cut, `/\`), separator: separator, disk: disk}, true
}

// childPath returns the path of names below directory, or false when a
// name is not a plain file name (parser.Component refuses it). A name read
// from the evidence never leads outside the tree.
func childPath(directory string, names ...string) (string, bool) {
	for _, name := range names {
		if _, ok := parser.Component(name); !ok {
			return "", false
		}
	}
	return filepath.Join(append([]string{directory}, names...)...), true
}

// hasJSONSuffix reports whether a file name's suffix, as Python's
// PurePath.suffix gives it (leading dots are not one), is ".json".
func hasJSONSuffix(name string) bool {
	return strings.HasSuffix(strings.TrimLeft(name, "."), ".json")
}

// jsonStem returns a file name without its ".json" suffix.
func jsonStem(name string) string {
	if hasJSONSuffix(name) {
		return strings.TrimSuffix(name, ".json")
	}
	return name
}

// children returns the names of the regular .json files of directory,
// sorted; none when it cannot be listed or is a link.
func children(directory string) []string {
	entries, err := parser.ReadDir(directory)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if hasJSONSuffix(entry.Name()) && parser.RegularFile(filepath.Join(directory, entry.Name())) {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names
}

// subdirectories returns the names of the directories of directory that
// are not links, sorted; none when it cannot be listed or is a link.
func subdirectories(directory string) []string {
	entries, err := parser.ReadDir(directory)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsDir() {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names
}

// quietObject returns a file read beside the one parsed decoded as a JSON
// object, or false; that file's own problems are reported where it is
// parsed, so none is kept here.
func quietObject(path string, ok bool) (jsonvalue.Value, bool) {
	if !ok {
		return jsonvalue.NewNull(), false
	}
	var ignored parser.Problems
	return readObject(path, &ignored)
}

// sessionScope returns a message file's session scope. The session file is
// info/<id>.json (old layout) or session/<project>/<id>.json, the first
// that decodes; the project path is its directory, else (new layout) its
// project file's worktree unless "/", else the message's path.root or
// path.cwd.
func sessionScope(root string, sessionID string, old bool, message jsonvalue.Value) *scope {
	session := jsonvalue.NewNull()
	if old {
		session, _ = quietObject(childPath(root, "info", sessionID+".json"))
	} else {
		for _, directory := range subdirectories(filepath.Join(root, "session")) {
			candidate, ok := childPath(filepath.Join(root, "session", directory), sessionID+".json")
			if !ok || !parser.RegularFile(candidate) {
				continue
			}
			if found, decoded := quietObject(candidate, true); decoded {
				session = found
				break
			}
		}
	}
	projectPath := parser.Str(session.Get("directory"))
	if projectID := session.Get("projectID"); projectPath == "" && !old && !projectID.IsEmpty() {
		project, _ := quietObject(childPath(root, "project", projectID.Text()+".json"))
		projectPath = worktreeText(project.Get("worktree"))
	}
	if projectPath == "" {
		where := objectOf(message.Get("path"))
		projectPath = parser.Str(firstTruthy(where.Get("root"), where.Get("cwd")))
	}
	return newScope(sessionID, projectPath)
}

// worktreeText returns a project's worktree as text, "" when it is "/".
func worktreeText(worktree jsonvalue.Value) string {
	if isText(worktree, "/") {
		return ""
	}
	return parser.Str(worktree)
}

// sessionFile emits the "session start:" row of a legacy session file and
// a row per problem; a file that does not decode as an object is one
// `parser:` row.
func (p *Parser) sessionFile(artifact *input.Artifact, old bool, emit parser.Emit) {
	var problems parser.Problems
	stem := jsonStem(artifact.Rel[strings.LastIndex(artifact.Rel, "/")+1:])
	session, ok := readObject(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, stem, ""))
		return
	}
	if !ok {
		row := p.SystemRow(artifact, "parser: session file could not be decoded")
		row.SourceLine, row.SessionID = 1, stem
		emit(row)
		return
	}
	projectPath := parser.Str(session.Get("directory"))
	if projectID := session.Get("projectID"); projectPath == "" && !old && !projectID.IsEmpty() {
		name := projectID.Text() + ".json"
		var file string
		root, ok := rootOf(artifact, 3)
		if ok {
			file, ok = childPath(root.disk, "project", name)
		}
		project, reason := readSide(file, ok, &problems)
		if reason != "" {
			problems.Add(1, fmt.Sprintf("project file %s could not be read (%s)", name, reason))
		}
		projectPath = worktreeText(project.Get("worktree"))
	}
	identifier := parser.Str(session.Get("id"))
	if identifier == "" {
		identifier = stem
	}
	scoped := newScope(identifier, projectPath)
	emit(p.sessionRow(artifact, scoped, jsonLookup(session),
		timeutil.FromValue(objectOf(session.Get("time")).Get("created")), 1, ""))
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, scoped.sessionID, scoped.projectPath))
}

// readSide returns a JSON file read beside the one parsed, decoded as an
// object, and why it is not one. A file that is absent, a link or not a
// regular file is silent; one that cannot be read adds its unreadable
// problem and gives no reason.
func readSide(path string, ok bool, problems *parser.Problems) (jsonvalue.Value, string) {
	if !ok || !parser.RegularFile(path) {
		return jsonvalue.NewNull(), ""
	}
	data, read := parser.FileBytes(path, problems)
	if !read {
		return jsonvalue.NewNull(), ""
	}
	value, reason, _ := decodeText(pytext.DecodeUTF8(data))
	return value, reason
}

// messageFile emits the rows of a legacy message file and its part files:
// storage/message/<session>/<message>.json with parts under
// storage/part/<message>/, or (old layout) storage/session/message/
// <session>/<message>.json with parts under storage/session/part/
// <session>/<message>/.
func (p *Parser) messageFile(artifact *input.Artifact, options parser.Options, old bool,
	emit parser.Emit) {
	root, rooted := rootOf(artifact, 3)
	if !rooted {
		return
	}
	parts := strings.Split(artifact.Rel, "/")
	sessionID, name := parts[len(parts)-2], parts[len(parts)-1]
	var problems parser.Problems
	message, ok := readObject(artifact.DiskPath, &problems)
	if !ok {
		scoped := sessionScope(root.disk, sessionID, old, jsonvalue.NewNull())
		if len(problems) > 0 {
			parser.EmitAll(emit, p.ErrorRows(artifact, problems, scoped.sessionID, scoped.projectPath))
			return
		}
		emit(p.systemRow(artifact, scoped, 1, "", "parser: message file could not be decoded"))
		return
	}
	if identifier := parser.Str(message.Get("sessionID")); identifier != "" {
		sessionID = identifier
	}
	messageID := parser.Str(message.Get("id"))
	if messageID == "" {
		messageID = jsonStem(name)
	}
	scoped := sessionScope(root.disk, sessionID, old, message)
	if previous, ok := previousMessage(parser.Beside(artifact), name); ok {
		provider, model := messageRef(previous)
		modelRowText(scoped.models, scoped.sessionID, provider, model)
		agentRowText(scoped.modes, scoped.sessionID, previous.Get("agent"))
	}
	partNames := []string{"part", messageID}
	if old {
		partNames = []string{"part", sessionID, messageID}
	}
	found, failures := root.partFiles(partNames)
	p.messageRows(artifact, scoped, message, origin{line: 1}, found, options, failures, emit)
}

// previousMessage returns the message file before the named one in its
// directory, decoded: one file holds one message, and message ids sort in
// time order, so the model and agent in force come from it.
func previousMessage(directory, name string) (jsonvalue.Value, bool) {
	siblings := children(directory)
	position := slices.Index(siblings, name)
	if position <= 0 {
		return jsonvalue.NewNull(), false
	}
	return quietObject(filepath.Join(directory, siblings[position-1]), true)
}

// partFiles returns the decoded part files of the directory names leads
// to below the root, each with its original path, and a failure per part
// file that cannot be read or decoded.
func (r treeRoot) partFiles(names []string) ([]part, []failure) {
	directory, ok := childPath(r.disk, names...)
	if !ok {
		return nil, nil
	}
	var found []part
	var failures []failure
	for _, file := range children(directory) {
		source := r.original + r.separator + strings.Join(append(slices.Clone(names), file), r.separator)
		var problems parser.Problems
		data, read := parser.FileBytes(filepath.Join(directory, file), &problems)
		if !read {
			for _, problem := range problems {
				failures = append(failures, failure{line: problem.Line, message: problem.Message,
					sourceFile: source})
			}
			continue
		}
		value, reason, raw := decodeText(pytext.DecodeUTF8(data))
		if reason != "" {
			failures = append(failures, failure{line: 1, raw: raw, sourceFile: source,
				message: fmt.Sprintf("unparseable part %s (%s)", jsonStem(file), reason)})
			continue
		}
		found = append(found, part{data: value, where: origin{file: source, line: 1}})
	}
	return found, failures
}
