package timeline

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/secrets/urlcredentials"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// CanonicalProjectPath returns one spelling for a Windows project path, a
// path as the evidence's machine wrote it (evidencepath): an upper-case
// drive letter, backslashes, no repeated or trailing separator and no \\?\
// or \\?\UNC\ prefix, the case after the drive kept, and the text from a
// URL with a credential on as written (asWrittenFrom). Any other path is
// returned unchanged: //server/share, and a UNC path whose server is "?"
// with more after it, which would read as an extended-length prefix.
func CanonicalProjectPath(path string) string {
	if canonical, ok := windowsForm(path); ok {
		return canonical
	}
	return path
}

// ProjectKey returns the key under which the inventory groups a project
// path: a Windows form canonicalised and lower-cased in ASCII, since
// Windows file names are case-insensitive, and any other path unchanged.
func ProjectKey(path string) string {
	if canonical, ok := windowsForm(path); ok {
		return textlines.LowerASCII(canonical)
	}
	return path
}

// IsWindowsPath reports whether path is in a Windows form that
// CanonicalProjectPath rewrites: a drive letter with a separator, or a UNC
// path naming at least a server, with or without an extended-length prefix.
func IsWindowsPath(path string) bool {
	_, ok := windowsForm(path)
	return ok
}

// windowsForm returns path in the canonical Windows form and true, or false
// when path is not a Windows form. A backslash is a legal character in a
// POSIX file name, so a path is read as a Windows one only in these forms.
func windowsForm(path string) (string, bool) {
	if rest, ok := strings.CutPrefix(path, `\\?\`); ok {
		path = rest
		if len(rest) >= 4 && strings.EqualFold(rest[:4], `UNC\`) {
			path = `\\` + rest[4:]
		}
	}
	if evidencepath.IsDrivePath(path) {
		return canonicalAfter(string(upperASCII(path[0]))+`:\`, path, 2), true
	}
	if rest, ok := strings.CutPrefix(path, `\\`); ok {
		parts := evidencepath.Elements(rest, true)
		if len(parts) == 1 || len(parts) > 1 && parts[0] != "?" {
			return canonicalAfter(`\\`, path, 2), true
		}
	}
	return "", false
}

// canonicalAfter returns path, a Windows form whose first from bytes are
// its drive or its \\, as head and then its elements joined by
// backslashes, up to where asWrittenFrom says to keep the path as written.
func canonicalAfter(head, path string, from int) string {
	kept := asWrittenFrom(path)
	if kept < from {
		return path
	}
	joined := strings.Join(evidencepath.Elements(path[from:kept], true), `\`)
	if kept == len(path) || joined == "" {
		return head + joined + path[kept:]
	}
	return head + joined + `\` + path[kept:]
}

// KeptAsWrittenFrom returns where CanonicalProjectPath keeps path, in its
// canonical form, as written: the element of its first URL when it is a
// Windows form holding a URL credential (asWrittenFrom), else its length.
func KeptAsWrittenFrom(path string) int {
	if !IsWindowsPath(path) {
		return len(path)
	}
	return asWrittenFrom(path)
}

// asWrittenFrom returns where path keeps its text as written: at the start
// of the element that holds its first scheme separator, when the path holds
// a URL credential the output replaces, else at its end. Rewriting a URL's
// slashes would leave no URL for the output to find the credential in. No
// Windows folder name holds a colon, so no real path is spelled differently.
func asWrittenFrom(path string) int {
	if len(secrets.URLCredentialRanges(path, false)) == 0 {
		return len(path)
	}
	separator, _ := urlcredentials.FirstSeparator(path)
	if separator < 0 {
		separator = len(path)
	}
	return strings.LastIndexAny(path[:separator], `/\`) + 1
}

// upperASCII returns char in upper case when it is an ASCII letter.
func upperASCII(char byte) byte {
	if char >= 'a' && char <= 'z' {
		return char - ('a' - 'A')
	}
	return char
}
