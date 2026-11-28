package opencode

import (
	"path"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
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
	disk, found := parserkit.Above(artifact, depth-1)
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

// hasJSONSuffix reports whether a file name ends in the extension ".json"
// in any ASCII case; a name of nothing but dots before it, such as ".json",
// is a dot file without one (parserkit.Stem).
func hasJSONSuffix(name string) bool {
	return textlines.EqualFoldASCII(path.Ext(name), ".json") && parserkit.Stem(name) != name
}

// jsonStem returns a file name without its ".json" extension, in any ASCII
// case, as the kinds match it.
func jsonStem(name string) string {
	if hasJSONSuffix(name) {
		return parserkit.Stem(name)
	}
	return name
}

// children returns the names of the regular .json files of directory,
// sorted; none when it cannot be listed.
func children(directory parserkit.Directory) []string {
	entries, err := parserkit.ReadDir(directory.Path())
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if !hasJSONSuffix(entry.Name()) {
			continue
		}
		if path, ok := directory.Child(entry.Name()); ok && parserkit.RegularFile(path) {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names
}

// subdirectories returns the names of the directories of directory that
// are not links, sorted; none when it cannot be listed or is a link.
func subdirectories(directory string) []string {
	entries, err := parserkit.ReadDir(directory)
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
func quietObject(artifact *input.Artifact, path string, ok bool) (jsonvalue.Value, bool) {
	if !ok {
		return jsonvalue.NewNull(), false
	}
	var ignored parserkit.Problems
	return readObject(artifact, path, &ignored)
}

// worktreeText returns a project's worktree as text, "" when it is "/".
func worktreeText(worktree jsonvalue.Value) string {
	if parserkit.IsText(worktree, "/") {
		return ""
	}
	return parserkit.Text(worktree)
}

// sessionProject returns the project path a session's side files give:
// its session file's directory, else (new layout) its project file's
// worktree unless "/", else "". It is read once per session for the home,
// since every message file of the session looks it up.
func (p *Parser) sessionProject(artifact *input.Artifact, tree *treeIndex, root string, sessionID string,
	old bool) string {
	key := projectKey{root: root, sessionID: sessionID, old: old}
	if projectPath, ok := tree.projectPaths[key]; ok {
		return projectPath
	}
	p.Work.Add("session side reads", 1)
	session := jsonvalue.NewNull()
	if old {
		info, found := parserkit.Under(artifact, root, "info", sessionID+".json")
		session = tree.quietSide(info, found, p.Work)
	} else if directory, ok := parserkit.Under(artifact, root, "session"); ok {
		session = tree.session(artifact, directory, sessionID, p.Work)
	}
	projectPath := parserkit.Text(session.Get("directory"))
	if projectID := session.Get("projectID"); projectPath == "" && !old && !projectID.IsEmpty() {
		file, found := parserkit.Under(artifact, root, "project", projectID.Text()+".json")
		project := tree.quietSide(file, found, p.Work)
		projectPath = worktreeText(project.Get("worktree"))
	}
	tree.projectPaths[key] = projectPath
	return projectPath
}

// sessionScope returns a message file's session scope. The session file is
// info/<id>.json (old layout) or session/<project>/<id>.json, the first
// that decodes; the project path is its directory, else (new layout) its
// project file's worktree unless "/", else the message's path.root or
// path.cwd.
func (p *Parser) sessionScope(artifact *input.Artifact, tree *treeIndex, root string, sessionID string, old bool,
	message jsonvalue.Value) *scope {
	projectPath := p.sessionProject(artifact, tree, root, sessionID, old)
	if projectPath == "" {
		where := parserkit.ObjectOrEmpty(message.Get("path"))
		projectPath = parserkit.Text(parserkit.FirstNonEmpty(where.Get("root"), where.Get("cwd")))
	}
	return newScope(sessionID, projectPath)
}

// sessionFile emits the "session start:" row of a legacy session file and
// a row per problem; a file that does not decode as an object is one
// `parser:` row.
func (p *Parser) sessionFile(artifact *input.Artifact, old bool, emit parserkit.Emit) {
	var problems parserkit.Problems
	stem := jsonStem(artifact.Rel[strings.LastIndex(artifact.Rel, "/")+1:])
	session, ok := readObject(artifact, artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, stem, ""))
		return
	}
	if !ok {
		row := p.SystemRow(artifact, "parser: session file could not be decoded")
		row.SourceLine, row.SessionID = 1, stem
		emit(row)
		return
	}
	projectPath := parserkit.Text(session.Get("directory"))
	if projectID := session.Get("projectID"); projectPath == "" && !old && !projectID.IsEmpty() {
		name := projectID.Text() + ".json"
		var file string
		root, ok := rootOf(artifact, 3)
		if ok {
			file, ok = parserkit.Under(artifact, root.disk, "project", name)
		}
		project, err := p.trees.For(artifact.Home).projectFile(file, ok, &problems, p.Work)
		if err != nil {
			problems.Add(1, "project file "+name+" could not be read ("+err.Error()+")")
		}
		projectPath = worktreeText(project.Get("worktree"))
	}
	identifier := parserkit.Text(session.Get("id"))
	if identifier == "" {
		identifier = stem
	}
	scoped := newScope(identifier, projectPath)
	emit(p.sessionRow(artifact, scoped, jsonLookup(session),
		timeutil.FromValue(parserkit.ObjectOrEmpty(session.Get("time")).Get("created")), 1, ""))
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, scoped.sessionID, scoped.projectPath))
}

// readSide returns a JSON file read beside the one parsed, decoded as an
// object, with an error saying why it is not one. A file that is absent,
// a link or not a regular file is silent; one that cannot be read adds its
// unreadable problem and gives no error.
func readSide(artifact *input.Artifact, path string, ok bool, problems *parserkit.Problems) (jsonvalue.Value, error) {
	if !ok || !parserkit.RegularFile(path) {
		return jsonvalue.NewNull(), nil
	}
	data, read := parserkit.FileBytes(path, problems)
	if !read {
		return jsonvalue.NewNull(), nil
	}
	return decodeText(artifact, textlines.ValidUTF8(data))
}

// partFiles returns the decoded part files of the directory names leads
// to below the root, each with its original path, and a failure per part
// file that cannot be read or decoded.
func (r treeRoot) partFiles(artifact *input.Artifact, names []string) ([]part, []failure) {
	path, ok := parserkit.Under(artifact, r.disk, names...)
	if !ok {
		return nil, nil
	}
	directory, ok := parserkit.DirectoryAt(artifact, path)
	if !ok {
		return nil, nil
	}
	var found []part
	var failures []failure
	for _, file := range children(directory) {
		source := r.original + r.separator + strings.Join(append(slices.Clone(names), file), r.separator)
		var problems parserkit.Problems
		data, read := []byte(nil), false
		if path, named := directory.Child(file); named {
			data, read = parserkit.FileBytes(path, &problems)
		}
		if !read {
			for _, problem := range problems {
				failures = append(failures, failure{line: problem.Line, message: problem.Message,
					sourceFile: source})
			}
			continue
		}
		raw := textlines.ValidUTF8(data)
		value, err := decodeText(artifact, raw)
		if err != nil {
			failures = append(failures, failure{line: 1, raw: raw, sourceFile: source,
				message: "unparseable part " + jsonStem(file) + " (" + err.Error() + ")"})
			continue
		}
		found = append(found, part{data: value, where: origin{file: source, line: 1}})
	}
	return found, failures
}

// messageParts returns the parts of a message file as partFiles does. A
// message id is unique, so another message file claiming the same id as
// the first file of the home whose part directory held any is damage: it
// gets one problem in place of the parts, so many such files read the
// directory once, whatever spelling of the id opens it. The first file
// parsed again reads them again.
func (p *Parser) messageParts(artifact *input.Artifact, root treeRoot, partNames []string,
	messageID string) ([]part, []failure) {
	directory, _ := parserkit.Under(artifact, root.disk, partNames...)
	key, tree := parserkit.KeyOf(directory), p.trees.For(artifact.Home)
	if first, ok := tree.partsShown[key]; ok && first.disk != artifact.DiskPath {
		return nil, []failure{{line: 1, message: "message " + messageID +
			" repeats the id of an earlier message file; its parts are shown with " + first.original}}
	}
	found, failures := root.partFiles(artifact, partNames)
	if len(found) > 0 || len(failures) > 0 {
		tree.partsShown[key] = shownBy{disk: artifact.DiskPath, original: artifact.Original}
	}
	return found, failures
}

// previousMessage returns the message file before the named one in its
// directory, decoded: one file holds one message, and message ids sort in
// time order, so the model and agent in force come from it.
func (p *Parser) previousMessage(artifact *input.Artifact, tree *treeIndex, directory, name string,
) (jsonvalue.Value, bool) {
	siblings := tree.messageNames(artifact, directory, p.Work)
	position, found := slices.BinarySearch(siblings, name)
	if !found || position == 0 {
		return jsonvalue.NewNull(), false
	}
	path, found := parserkit.Under(artifact, directory, siblings[position-1])
	// The previous message is its own artifact, whose parse reports its
	// repeated member names.
	return quietObject(nil, path, found)
}

// messageFile emits the rows of a legacy message file and its part files:
// storage/message/<session>/<message>.json with parts under
// storage/part/<message>/, or (old layout) storage/session/message/
// <session>/<message>.json with parts under storage/session/part/
// <session>/<message>/.
func (p *Parser) messageFile(artifact *input.Artifact, options parserkit.Options, old bool,
	emit parserkit.Emit) {
	root, rooted := rootOf(artifact, 3)
	if !rooted {
		return
	}
	parts := strings.Split(artifact.Rel, "/")
	sessionID, name := parts[len(parts)-2], parts[len(parts)-1]
	tree := p.trees.For(artifact.Home)
	var problems parserkit.Problems
	message, ok := readObject(artifact, artifact.DiskPath, &problems)
	if !ok {
		scoped := p.sessionScope(artifact, tree, root.disk, sessionID, old, jsonvalue.NewNull())
		if len(problems) > 0 {
			parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, scoped.sessionID, scoped.projectPath))
			return
		}
		emit(p.systemRow(artifact, scoped, 1, "", "parser: message file could not be decoded"))
		return
	}
	if identifier := parserkit.Text(message.Get("sessionID")); identifier != "" {
		sessionID = identifier
	}
	messageID := parserkit.Text(message.Get("id"))
	if messageID == "" {
		messageID = jsonStem(name)
	}
	scoped := p.sessionScope(artifact, tree, root.disk, sessionID, old, message)
	if directory, found := parserkit.Beside(artifact); found {
		if previous, ok := p.previousMessage(artifact, tree, directory, name); ok {
			provider, model := messageRef(previous)
			modelRowText(scoped.models, scoped.sessionID, provider, model)
			agentRowText(scoped.modes, scoped.sessionID, previous.Get("agent"))
		}
	}
	partNames := []string{"part", messageID}
	if old {
		partNames = []string{"part", sessionID, messageID}
	}
	found, failures := p.messageParts(artifact, root, partNames, messageID)
	p.messageRows(artifact, scoped, message, origin{line: 1}, found, options, failures, nil, emit)
}
