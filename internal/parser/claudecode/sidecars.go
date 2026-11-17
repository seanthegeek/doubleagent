package claudecode

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/reason"
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

// isLink reports whether path's own entry is a symbolic link.
func isLink(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}

// withoutSuffix returns a file name without its suffix, the part from its
// last dot when the dot is neither first nor last.
func withoutSuffix(name string) string {
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 || dot == len(name)-1 {
		return name
	}
	return name[:dot]
}

// sessionDir returns the directory that holds a transcript's sidecars:
// <session>/ beside a session file, or the directory above subagents/ for a
// subagent's file. It is false when no subagents/ directory lies below the
// artifact's home.
func sessionDir(artifact *input.Artifact) (string, bool) {
	if !subagentPattern.MatchString(artifact.Rel) {
		return parser.Beside(artifact, withoutSuffix(filepath.Base(artifact.DiskPath))), true
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

// persistedText returns the full text of a persisted tool output, read
// from the session's tool-results/ directory. savedPath is the line's
// toolUseResult.persistedOutputPath; when it is empty the path is read from
// the stub's text. The stub comes back with a note when the file is not in
// this session's tool-results/ or is a link, and with the reason when it is
// there but cannot be read, the unreadable problem added to problems.
func persistedText(artifact *input.Artifact, stub, savedPath string, problems *parser.Problems) string {
	if savedPath == "" {
		match := persistedPattern.FindStringSubmatch(stub)
		if match == nil {
			return stub
		}
		savedPath = match[1]
	}
	parts := pathSeparators.Split(savedPath, -1)
	name := parts[len(parts)-1]
	directory, found := sessionDir(artifact)
	note := stub + "\n[persisted output not found: tool-results/" + name + "]"
	_, named := parser.Component(name)
	if len(parts) < 3 || parts[len(parts)-2] != "tool-results" ||
		!found || parts[len(parts)-3] != filepath.Base(directory) || !named {
		return note
	}
	results := filepath.Join(directory, "tool-results")
	output := filepath.Join(results, name)
	if isLink(directory) || isLink(results) || !parser.RegularFile(output) {
		return note
	}
	data, err := parser.ReadFile(output)
	if err != nil {
		problems.AddUnreadable(0, output, err)
		return stub + "\n[persisted output could not be read (" + reason.For(err) + ")]"
	}
	return pytext.DecodeUTF8(data)
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
	cache := filepath.Join(historyDir, "paste-cache")
	cacheFile := filepath.Join(cache, hash+".txt")
	if isLink(cache) || !parser.RegularFile(cacheFile) {
		return "", false
	}
	data, err := parser.ReadFile(cacheFile)
	if err != nil {
		problems.AddUnreadable(0, cacheFile, err)
		return "", false
	}
	return pytext.DecodeUTF8(data), true
}

// expandPastes returns a history line's display text with each paste
// placeholder replaced by its text: pastedContents.<n>.content when the
// paste is kept inline, else paste-cache/<contentHash>.txt. A placeholder
// whose text is in neither place is kept as written.
func expandPastes(historyDir, display string, pasted jsonvalue.Value, problems *parser.Problems) string {
	if !pasted.IsObject() || pasted.Len() == 0 {
		return display
	}
	var out strings.Builder
	last := 0
	for _, match := range pastePattern.FindAllStringSubmatchIndex(display, -1) {
		out.WriteString(display[last:match[0]])
		placeholder := display[match[0]:match[1]]
		if text, ok := pasteText(historyDir, pasted.Get(display[match[2]:match[3]]), problems); ok {
			out.WriteString(text)
		} else {
			out.WriteString(placeholder)
		}
		last = match[1]
	}
	out.WriteString(display[last:])
	return out.String()
}

// pasteText returns the text of one pastedContents entry, or false when the
// entry is not a text paste or its text is in neither place.
func pasteText(historyDir string, entry jsonvalue.Value, problems *parser.Problems) (string, bool) {
	if !entry.IsObject() {
		return "", false
	}
	if kind, ok := entry.Lookup("type"); ok && !isText(kind, "text") {
		return "", false
	}
	if content := entry.Get("content"); content.IsString() {
		return content.StringValue(), true
	}
	if hash := entry.Get("contentHash"); hash.IsString() {
		return readPaste(historyDir, hash.StringValue(), problems)
	}
	return "", false
}
