package claudecode

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
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
		return parserkit.Beside(artifact, parserkit.Stem(filepath.Base(artifact.DiskPath)))
	}
	for levels := 0; ; levels++ {
		directory, ok := parserkit.Above(artifact, levels)
		if !ok {
			return "", false
		}
		if textlines.EqualFoldASCII(filepath.Base(directory), "subagents") {
			return parserkit.Above(artifact, levels+1)
		}
	}
}

// persistedFile is what was read of one persisted tool output: its text,
// shared by every row naming it, or the error reading it.
type persistedFile struct {
	shared *timeline.SharedText
	err    error
}

// persistedText adds to out the full text of a persisted tool output, read
// from the session's tool-results/ directory. savedPath is the line's
// toolUseResult.persistedOutputPath; when it is empty the path is read from
// the stub's text. The stub is added with a note when the file is not in
// this session's tool-results/ or is a link, and with the reason when it is
// there but cannot be read, the unreadable problem added. Each file is read
// once for all the transcripts of its home, by whatever spelling of its
// name opens it (Reader.outputs), and every row naming it shares its text:
// the output writes it in full in the first of its rows it keeps, and in
// each later one the stub with a note naming the row above that holds it.
func (s *session) persistedText(out *timeline.TextBuilder, stub, savedPath string) {
	if savedPath == "" {
		match := persistedPattern.FindStringSubmatch(stub)
		if match == nil {
			out.Write(stub)
			return
		}
		savedPath = match[1]
	}
	parts := pathSeparators.Split(savedPath, -1)
	name := parts[len(parts)-1]
	directory, found := sessionDir(s.artifact)
	note := stub + "\n[persisted output not found: tool-results/" + name + "]"
	_, named := parserkit.Component(name)
	if len(parts) < 3 || parts[len(parts)-2] != "tool-results" ||
		!found || parts[len(parts)-3] != filepath.Base(directory) || !named {
		out.Write(note)
		return
	}
	output, reachable := parserkit.Under(directory, "tool-results", name)
	if !reachable || !parserkit.RegularFile(output) {
		out.Write(note)
		return
	}
	files, key := s.reader.outputs.For(s.artifact.Home), parserkit.KeyOf(output)
	file, read := files[key]
	if !read {
		s.reader.Work.Add("persisted outputs read", 1)
		data, err := parserkit.ReadFile(output)
		file = &persistedFile{shared: timeline.NewSharedText(textlines.ValidUTF8(data)), err: err}
		files[key] = file
	}
	if file.err != nil {
		s.problems.AddUnreadable(0, output, file.err)
		out.Write(stub + "\n[persisted output could not be read (" + reason.For(file.err) + ")]")
		return
	}
	out.Share(file.shared, stub, persistedPointer(stub, name))
}

// persistedPointer returns the pointer of a later reference to the
// persisted output name: the stub, then a note naming the transcript and
// line of the row above that shows the output. The transcript's path is at
// most input.MaxPathLength bytes, the carried bound, so the note needs no
// cut.
func persistedPointer(stub, name string) timeline.Pointer {
	return func(holder *timeline.Row, _ bool) string {
		return stub + "\n[persisted output shown in full above (" + holder.SourceFile + " line " +
			strconv.Itoa(holder.SourceLine) + "): tool-results/" + name + "]"
	}
}

// persistedFiles holds the persisted tool outputs of one home read so
// far, by the entry each name opened. A home's session directories each
// hold their own tool-results/, so a file's entry names it within the home.
type persistedFiles map[parserkit.EntryKey]*persistedFile

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

// pasteFile returns the path of paste-cache/<hash>.txt in the history's
// directory, false for a name that is not a plain hash or a path that
// cannot be reached without a link.
func pasteFile(historyDir, hash string) (string, bool) {
	if !isPasteHash(hash) {
		return "", false
	}
	return parserkit.Under(historyDir, "paste-cache", hash+".txt")
}

// pasteEntry returns the key of the paste-cache file a hash names, the
// zero key when no path reaches one, since no file is then read.
func pasteEntry(historyDir, hash string) parserkit.EntryKey {
	cacheFile, reachable := pasteFile(historyDir, hash)
	if !reachable {
		return parserkit.EntryKey{}
	}
	return parserkit.KeyOf(cacheFile)
}

// readPaste returns the text of paste-cache/<hash>.txt in the history's
// directory, or false: silently for a name that is not a plain hash, a
// missing file or a link (never followed), and with the unreadable problem
// added for a file that is there but cannot be read, since a read failure
// is evidence.
func readPaste(historyDir, hash string, problems *parserkit.Problems) (string, bool) {
	cacheFile, reachable := pasteFile(historyDir, hash)
	if !reachable || !parserkit.RegularFile(cacheFile) {
		return "", false
	}
	data, err := parserkit.ReadFile(cacheFile)
	if err != nil {
		problems.AddUnreadable(0, cacheFile, err)
		return "", false
	}
	return textlines.ValidUTF8(data), true
}

// paste is the text of one paste-cache file, shared by every placeholder
// naming it, whether it was read, and the problems reading it added.
type paste struct {
	shared   *timeline.SharedText
	found    bool
	problems parserkit.Problems
}

// pasteFiles holds the paste-cache files of one home read so far, by the
// entry each hash opened, so every spelling of one file's name is one text.
type pasteFiles map[parserkit.EntryKey]*paste

// pasteOf returns the paste-cache file of hash in historyDir, read once for
// every line of the home naming it, by whatever spelling of its hash opens
// it; each load of an input makes its own home values, so a second pass
// over the same files reads it again.
func (r *Reader) pasteOf(home *input.Home, historyDir, hash string) *paste {
	files, key := r.pastes.For(home), pasteEntry(historyDir, hash)
	each, ok := files[key]
	if !ok {
		each = &paste{}
		var text string
		text, each.found = readPaste(historyDir, hash, &each.problems)
		each.shared = timeline.NewSharedText(text)
		files[key] = each
	}
	return each
}

// expandPastes adds to out a history line's display text with each paste
// placeholder standing for its text: pastedContents.<n>.content when the
// paste is kept inline, else paste-cache/<contentHash>.txt in historyDir. A
// placeholder whose text is in neither place is kept as written.
func (r *Reader) expandPastes(out *timeline.TextBuilder, home *input.Home, historyDir, display string,
	pasted jsonvalue.Value, problems *parserkit.Problems) {
	expandPastesFrom(out, display, pasted, problems, func(hash string) *paste {
		return r.pasteOf(home, historyDir, hash)
	})
}

// expandPastesFrom is expandPastes with the paste-cache file of a hash
// given by cached; the problems of its read are added at each placeholder
// naming it, as a fresh read would add them. A paste is one text for every
// placeholder that names it, by its number within the line for an inline
// paste and by the hash of its file for one in paste-cache/, across lines
// too: the output writes it in full at its first placeholder it keeps, and
// each later one as that placeholder with a note before its closing
// bracket saying where above it is shown (pastePointer).
func expandPastesFrom(out *timeline.TextBuilder, display string, pasted jsonvalue.Value,
	problems *parserkit.Problems, cached func(hash string) *paste) {
	if !pasted.IsObject() || pasted.Len() == 0 {
		out.Write(display)
		return
	}
	inline := map[string]*timeline.SharedText{}
	last := 0
	for _, match := range pastePattern.FindAllStringSubmatchIndex(display, -1) {
		out.Write(display[last:match[0]])
		placeholder := display[match[0]:match[1]]
		number := display[match[2]:match[3]]
		if shared, ok := pasteText(number, pasted.Get(number), inline, problems, cached); ok {
			out.Share(shared, placeholder, pastePointer(placeholder))
		} else {
			out.Write(placeholder)
		}
		last = match[1]
	}
	out.Write(display[last:])
}

// pastePointer returns the pointer of a later placeholder of a paste: the
// placeholder with ", shown above" before its closing bracket, and the
// history file and line of the row showing it when that is another row.
// The history's path is at most input.MaxPathLength bytes, the carried
// bound, so the note needs no cut.
func pastePointer(placeholder string) timeline.Pointer {
	opening := strings.TrimSuffix(placeholder, "]")
	return func(holder *timeline.Row, sameRow bool) string {
		if sameRow {
			return opening + ", shown above]"
		}
		return opening + ", shown above in " + holder.SourceFile + " line " + strconv.Itoa(holder.SourceLine) + "]"
	}
}

// pasteText returns the shared text of the pastedContents entry of a paste
// number: from inline, by number, for a paste kept in the line, else its
// paste-cache file through cached, whose problems are added. It is false
// when the entry is not a text paste or its text is in neither place.
func pasteText(number string, entry jsonvalue.Value, inline map[string]*timeline.SharedText,
	problems *parserkit.Problems, cached func(hash string) *paste) (*timeline.SharedText, bool) {
	if !entry.IsObject() {
		return nil, false
	}
	if kind, ok := entry.Lookup("type"); ok && !parserkit.IsText(kind, "text") {
		return nil, false
	}
	if content := entry.Get("content"); content.IsString() {
		shared, ok := inline[number]
		if !ok {
			shared = timeline.NewSharedText(content.StringValue())
			inline[number] = shared
		}
		return shared, true
	}
	if hash := entry.Get("contentHash"); hash.IsString() {
		file := cached(hash.StringValue())
		*problems = append(*problems, file.problems...)
		return file.shared, file.found
	}
	return nil, false
}
