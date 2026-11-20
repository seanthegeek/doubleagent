package claudecode

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// persistedMark starts the stub a transcript keeps in place of a tool
// output saved to its own file, and persistedPattern finds that file's path
// in the stub.
const persistedMark = "<persisted-output>"

var (
	persistedPattern = regexp.MustCompile(
		`Full output saved to: ([^\r\n]*?tool-results[/\\][^/\\\r\n]+?\.txt)`)
	pathSeparators = regexp.MustCompile(`[/\\]`)
	// A history line's paste placeholder, [Pasted text #<n> +<m> lines] or
	// [Pasted text #<n>].
	pastePattern = regexp.MustCompile(`\[Pasted text #([0-9]+)(?: \+[0-9]+ lines)?\]`)
)

// sessionDir returns the directory that holds a transcript's sidecars:
// <session>/ beside a session file, or the directory above subagents/ for a
// subagent's file. It is false when no subagents/ directory lies below the
// artifact's home.
func sessionDir(artifact *input.Artifact) (string, bool) {
	if !subagentPattern.MatchString(artifact.Rel) {
		return parser.Beside(artifact, parser.Stem(filepath.Base(artifact.DiskPath)))
	}
	for levels := 0; ; levels++ {
		directory, ok := parser.Above(artifact, levels)
		if !ok {
			return "", false
		}
		if filepath.Base(directory) == "subagents" {
			return parser.Above(artifact, levels+1)
		}
	}
}

// persistedFile is what was read of one persisted tool output: its text
// until a row shows it, or the error reading it, and once shown the
// transcript and line of that row.
type persistedFile struct {
	text    string
	err     error
	shownAt string
}

// persistedText returns the full text of a persisted tool output, read
// from the session's tool-results/ directory. savedPath is the line's
// toolUseResult.persistedOutputPath; when it is empty the path is read from
// the stub's text. The stub comes back with a note when the file is not in
// this session's tool-results/ or is a link, and with the reason when it is
// there but cannot be read, the unreadable problem added. Each file is read
// once for all the session's transcripts of a loaded input
// (Reader.outputsOf): its first reference in parse order shows the text,
// and every later one the stub with a note naming the transcript and line
// of the row that shows it.
func (s *session) persistedText(stub, savedPath string) string {
	if savedPath == "" {
		match := persistedPattern.FindStringSubmatch(stub)
		if match == nil {
			return stub
		}
		savedPath = match[1]
	}
	parts := pathSeparators.Split(savedPath, -1)
	name := parts[len(parts)-1]
	directory, found := sessionDir(s.artifact)
	note := stub + "\n[persisted output not found: tool-results/" + name + "]"
	_, named := parser.Component(name)
	if len(parts) < 3 || parts[len(parts)-2] != "tool-results" ||
		!found || parts[len(parts)-3] != filepath.Base(directory) || !named {
		return note
	}
	output, reachable := parser.Under(directory, "tool-results", name)
	if !reachable || !parser.RegularFile(output) {
		return note
	}
	files := s.reader.outputsOf(s.artifact.Home, directory)
	file, read := files[name]
	if !read {
		s.reader.Work.Add("persisted outputs read", 1)
		data, err := parser.ReadFile(output)
		file = &persistedFile{text: textlines.ValidUTF8(data), err: err}
		files[name] = file
	}
	switch {
	case file.err != nil:
		s.problems.AddUnreadable(0, output, file.err)
		return stub + "\n[persisted output could not be read (" + reason.For(file.err) + ")]"
	case file.shownAt != "":
		return stub + "\n[persisted output shown in full at its first reference (" + file.shownAt +
			"): tool-results/" + name + "]"
	}
	// The transcript's path is at most input.MaxPathLength bytes, the
	// carried bound, so the note needs no cut.
	file.shownAt = s.artifact.Original + " line " + strconv.Itoa(s.line)
	text := file.text
	file.text = ""
	return text
}

// outputsOf returns the persisted tool outputs read so far for the
// session directory of home, by file name. They are kept for the pass over
// the home's files, since a session's subagent transcripts are files of
// their own that share its tool-results/; each load of an input makes its
// own home values, so a second pass over the same files starts afresh.
func (r *Reader) outputsOf(home *input.Home, directory string) map[string]*persistedFile {
	key := outputsKey{home: home, directory: directory}
	if r.outputs == nil {
		r.outputs = map[outputsKey]map[string]*persistedFile{}
	}
	files, ok := r.outputs[key]
	if !ok {
		files = map[string]*persistedFile{}
		r.outputs[key] = files
	}
	return files
}

// outputsKey names a session directory of one home of a loaded input.
type outputsKey struct {
	home      *input.Home
	directory string
}

// isPasteHash reports whether a paste's content hash is a plain name of
// ASCII letters and digits, the only names read from paste-cache/.
func isPasteHash(hash string) bool {
	for i := range len(hash) {
		character := hash[i]
		if !('0' <= character && character <= '9' || 'A' <= character && character <= 'Z' ||
			'a' <= character && character <= 'z') {
			return false
		}
	}
	return hash != ""
}

// readPaste returns the text of paste-cache/<hash>.txt in the history's
// directory, or false: silently for a name that is not a plain hash, a missing file or
// a link (never followed), and with the unreadable problem added for a file
// that is there but cannot be read, since a read failure is evidence.
func readPaste(historyDir, hash string, problems *parser.Problems) (string, bool) {
	if !isPasteHash(hash) {
		return "", false
	}
	cacheFile, reachable := parser.Under(historyDir, "paste-cache", hash+".txt")
	if !reachable || !parser.RegularFile(cacheFile) {
		return "", false
	}
	data, err := parser.ReadFile(cacheFile)
	if err != nil {
		problems.AddUnreadable(0, cacheFile, err)
		return "", false
	}
	return textlines.ValidUTF8(data), true
}

// paste is the text of one paste-cache file, whether it was read, and the
// problems reading it added.
type paste struct {
	text     string
	found    bool
	problems parser.Problems
}

// expandPastes returns a history line's display text with each paste
// placeholder replaced by its text: pastedContents.<n>.content when the
// paste is kept inline, else paste-cache/<contentHash>.txt in historyDir. A
// placeholder whose text is in neither place is kept as written.
func expandPastes(historyDir, display string, pasted jsonvalue.Value, problems *parser.Problems) string {
	return expandPastesFrom(display, pasted, problems, func(hash string, read *parser.Problems) (string, bool) {
		return readPaste(historyDir, hash, read)
	})
}

// expandPastesFrom is expandPastes with the paste-cache file of a hash read
// by readCache. Each file is read once per line, however many placeholders
// name it; the problems of that read are added for each of them, as a
// fresh read would. A paste the line names again, by its number or by the
// hash of its paste-cache file, is written in full at its first
// placeholder and as that placeholder with ", shown above" at each later
// one, so a paste named many times is written once.
func expandPastesFrom(display string, pasted jsonvalue.Value, problems *parser.Problems,
	readCache func(hash string, read *parser.Problems) (string, bool)) string {
	if !pasted.IsObject() || pasted.Len() == 0 {
		return display
	}
	files := map[string]*paste{}
	cached := func(hash string) (string, bool) {
		each, ok := files[hash]
		if !ok {
			each = &paste{}
			each.text, each.found = readCache(hash, &each.problems)
			files[hash] = each
		}
		*problems = append(*problems, each.problems...)
		return each.text, each.found
	}
	shown := map[string]bool{}
	var out strings.Builder
	last := 0
	for _, match := range pastePattern.FindAllStringSubmatchIndex(display, -1) {
		out.WriteString(display[last:match[0]])
		placeholder := display[match[0]:match[1]]
		number := display[match[2]:match[3]]
		text, identity, ok := pasteText(number, pasted.Get(number), cached)
		switch {
		case !ok:
			out.WriteString(placeholder)
		case shown[identity]:
			out.WriteString(strings.TrimSuffix(placeholder, "]") + ", shown above]")
		default:
			shown[identity] = true
			out.WriteString(text)
		}
		last = match[1]
	}
	out.WriteString(display[last:])
	return out.String()
}

// pasteText returns the text of the pastedContents entry of a paste
// number, its paste-cache file read through cached, and the paste's
// identity within the line: its number for an inline paste, its hash for
// one in paste-cache/. It is false when the entry is not a text paste or
// its text is in neither place.
func pasteText(number string, entry jsonvalue.Value, cached func(hash string) (string, bool)) (string, string,
	bool) {
	if !entry.IsObject() {
		return "", "", false
	}
	if kind, ok := entry.Lookup("type"); ok && !parser.IsText(kind, "text") {
		return "", "", false
	}
	if content := entry.Get("content"); content.IsString() {
		return content.StringValue(), "number " + number, true
	}
	if hash := entry.Get("contentHash"); hash.IsString() {
		text, ok := cached(hash.StringValue())
		return text, "hash " + hash.StringValue(), ok
	}
	return "", "", false
}
