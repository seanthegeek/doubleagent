package projects

import (
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
)

// The readers below match one line at a time, so a value never spans a
// newline; they parse no format. The keys are fixed names, quoted before
// they reach a pattern. A reader of JSON or JSON5 text decodes the \uXXXX
// escapes of each value it takes from between quotes before any other step
// (jsonStrings), since a JSON writer may write any character so; the path
// rules undo `\/`, and the drive rules also `\\`, later.

// groupValues returns the first parenthesised group of every match of
// pattern in line.
func groupValues(pattern *regexp.Regexp, line []byte) []string {
	var values []string
	for _, match := range pattern.FindAllSubmatch(line, -1) {
		values = append(values, string(match[1]))
	}
	return values
}

// jsonStrings replaces each value, the inside of a JSON string, by its
// decodeJSONEscapes text, and returns values.
func jsonStrings(values []string) []string {
	for i, value := range values {
		values[i] = decodeJSONEscapes(value)
	}
	return values
}

// decodeJSONEscapes returns text, the inside of a JSON string, with each
// \uXXXX escape (RFC 8259 section 7) turned into its character's UTF-8 bytes
// and a surrogate pair into one character. A lone surrogate stays as written,
// and so does an escape naming a backslash, which the path rules would read
// as the start of an escape; any other backslash is copied with the byte
// after it, so `\\u00e9` stays. A result holding a newline, which would
// split the one-value-a-line stream, or a NUL, which no path holds, gives
// text as written.
func decodeJSONEscapes(text string) string {
	if !strings.Contains(text, `\u`) {
		return text
	}
	decoded := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		code, ok := unicodeEscape(text, i)
		switch {
		case ok && utf16.IsSurrogate(code):
			low, lowOK := unicodeEscape(text, i+6)
			if pair := utf16.DecodeRune(code, low); lowOK && pair != utf8.RuneError {
				decoded = utf8.AppendRune(decoded, pair)
				i += 11
			} else {
				decoded = append(decoded, text[i:i+6]...)
				i += 5
			}
		case ok && code == '\\':
			decoded = append(decoded, text[i:i+6]...)
			i += 5
		case ok:
			decoded = utf8.AppendRune(decoded, code)
			i += 5
		case text[i] == '\\' && i+1 < len(text):
			decoded = append(decoded, text[i], text[i+1])
			i++
		default:
			decoded = append(decoded, text[i])
		}
	}
	if strings.ContainsAny(string(decoded), "\n\x00") {
		return text
	}
	return string(decoded)
}

// unicodeEscape returns the code a `\uXXXX` escape at position in text
// names, XX hexadecimal digits of either case, and false when none starts
// there.
func unicodeEscape(text string, position int) (rune, bool) {
	if position+6 > len(text) || text[position] != '\\' || text[position+1] != 'u' {
		return 0, false
	}
	var code rune
	for offset := 2; offset < 6; offset++ {
		digit, ok := hexDigit(text, position+offset)
		if !ok {
			return 0, false
		}
		code = code<<4 | rune(digit)
	}
	return code, true
}

// jsonValueMatcher matches the value of each `"KEY": "..."` in a line.
func jsonValueMatcher(key string) func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`"` + regexp.QuoteMeta(key) + `": *"([^"]*)"`)
	return func(line []byte) []string { return jsonStrings(groupValues(pattern, line)) }
}

// jsonPathKeyMatcher matches each quoted key starting with `/` (or, under
// the driveJSON rule, a drive letter and a colon) followed by a colon and
// next, a bracket expression or a literal byte.
func jsonPathKeyMatcher(next string, rules pathRules) func(line []byte) []string {
	key := `/[^"]*`
	if rules.driveJSON {
		key = `(/|[A-Za-z]:)[^"]*`
	}
	pattern := regexp.MustCompilePOSIX(`"(` + key + `)": *` + next)
	return func(line []byte) []string { return jsonStrings(groupValues(pattern, line)) }
}

// absStringMatcher matches every quoted absolute path or file:// URI in a
// line, turned into a path by localPath; under the driveJSON rule a drive
// path written with an escaped backslash too, turned by drivePathText, and
// a drive URI keeps its drive.
func absStringMatcher(rules pathRules) func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`"((file://)?/[^"]*)"`)
	if rules.driveJSON {
		pattern = regexp.MustCompilePOSIX(`"((file://)?/[^"]*|[A-Za-z]:\\\\[^"]*)"`)
	}
	return func(line []byte) []string {
		values := jsonStrings(groupValues(pattern, line))
		for i, value := range values {
			if evidencepath.IsDrivePath(value) {
				values[i] = drivePathText(value)
			} else {
				values[i] = localPath(value, rules.driveJSON)
			}
		}
		return values
	}
}

// drivePathText returns a drive path read from JSON text with `\\` and `\/`
// undone and every backslash a slash, so commonDir splits it.
func drivePathText(value string) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, `\\`, `\`), `\/`, "/")
	return strings.ReplaceAll(value, `\`, "/")
}

// localPath turns an absStringMatcher value into a path: a file:// URL loses
// its scheme and is decoded (decodeURLPath), and a drive URL (`file:///C:/x`,
// `file:///C%3A/x`) becomes its drive path (`C:/x`) when keepDrive is set,
// so the drive rules place it, else keeps only its path below the drive.
func localPath(value string, keepDrive bool) string {
	rest, isURL := strings.CutPrefix(value, "file://")
	if !isURL {
		return value
	}
	rest = decodeURLPath(rest)
	if len(rest) >= 4 && rest[0] == '/' && evidencepath.IsASCIILetter(rest[1]) &&
		rest[2] == ':' && rest[3] == '/' {
		if keepDrive {
			return rest[1:]
		}
		return rest[3:]
	}
	return rest
}

// decodeURLPath returns the path of a URL with every %XX escape decoded
// (percentDecode), or as written when the decoded path holds a newline,
// which the one-value-a-line stream would split into two paths.
func decodeURLPath(path string) string {
	if decoded := percentDecode(path); !strings.Contains(decoded, "\n") {
		return decoded
	}
	return path
}

// isQuote reports whether char is a double or a single quote.
func isQuote(char byte) bool { return char == '"' || char == '\'' }

// yamlValueMatcher gives the rest of a line starting with `KEY:`, less its
// leading blanks and one leading and one trailing quote, not required to pair.
func yamlValueMatcher(key string) func(line []byte) []string {
	return func(line []byte) []string {
		text, found := strings.CutPrefix(string(line), key+":")
		if !found {
			return nil
		}
		text = strings.TrimLeft(text, " ")
		if text != "" && isQuote(text[0]) {
			text = text[1:]
		}
		if text != "" && isQuote(text[len(text)-1]) {
			text = text[:len(text)-1]
		}
		return []string{text}
	}
}

// json5ValueMatcher matches the string value of KEY, bare or in either
// quote and not preceded by a name character. The prefix is cut at the last
// key, colon and quote in the match, so a value repeating them keeps only
// what follows.
func json5ValueMatcher(key string) func(line []byte) []string {
	quoted := regexp.QuoteMeta(key)
	pattern := regexp.MustCompilePOSIX(
		`(^|[^A-Za-z0-9_$])["']?` + quoted + `["']? *: *("[^"]*"|'[^']*')`)
	prefix := regexp.MustCompilePOSIX(`^.*` + quoted + `["']? *: *["']`)
	return func(line []byte) []string {
		var values []string
		for _, match := range pattern.FindAll(line, -1) {
			value := string(match)
			if span := prefix.FindStringIndex(value); span != nil {
				value = value[span[1]:]
			}
			if value != "" && isQuote(value[len(value)-1]) {
				value = decodeJSONEscapes(value[:len(value)-1])
			}
			values = append(values, value)
		}
		return values
	}
}

// tomlFileURLMatcher matches the decoded path of a `KEY = "file://..."` line
// (urlPaths).
func tomlFileURLMatcher(key string, rules pathRules) func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`^ *` + regexp.QuoteMeta(key) + ` *= *["']file://([^"']*)["']`)
	return func(line []byte) []string { return urlPaths(groupValues(pattern, line), rules) }
}

// sessionCwdMatcher matches each `"cwd":"..."`, with no blank after the colon.
func sessionCwdMatcher() func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`"cwd":"([^"]*)"`)
	return func(line []byte) []string { return jsonStrings(groupValues(pattern, line)) }
}

// workspaceFolderMatcher matches the decoded path of each
// `"folder": "file://..."` in a VS Code workspace.json (urlPaths).
func workspaceFolderMatcher(rules pathRules) func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`"folder": *"file://([^"]*)"`)
	return func(line []byte) []string {
		return urlPaths(jsonStrings(groupValues(pattern, line)), rules)
	}
}

// additionalDirectoryMatcher matches the quoted absolute paths inside each
// Agent Zero `"acp_additional_directories": [...]` list closed on its line.
func additionalDirectoryMatcher() func(line []byte) []string {
	list := regexp.MustCompilePOSIX(`"acp_additional_directories": *\[[^]]*\]`)
	item := regexp.MustCompilePOSIX(`"(/[^"]*)"`)
	return func(line []byte) []string {
		var values []string
		for _, match := range list.FindAll(line, -1) {
			values = append(values, jsonStrings(groupValues(item, match))...)
		}
		return values
	}
}

// urlPaths replaces each URL path, its file:// scheme cut, by its
// decodeURLPath, a drive URL's (`/C:/x`) by its drive path (`C:/x`) under
// the driveJSON rule, so the drive rules know it for a drive URL.
func urlPaths(values []string, rules pathRules) []string {
	for i, value := range values {
		values[i] = decodeURLPath(value)
		if rules.driveJSON {
			values[i] = driveFromURIPath(values[i])
		}
	}
	return values
}

// gooseWorkingDirs returns the items of a Goose project file's `workingDirs:`
// list, less trailing blanks, a CR and one pair of quotes, an empty one
// skipped; any other line ends the list.
func gooseWorkingDirs(lines []string) []string {
	var items []string
	listed := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		switch {
		case line == "workingDirs:":
			listed = true
		case listed && (line == "-" || strings.HasPrefix(line, "- ")):
			item := line[min(2, len(line)):]
			if len(item) >= 2 && isQuote(item[0]) && item[len(item)-1] == item[0] {
				item = item[1 : len(item)-1]
			}
			if item != "" {
				items = append(items, item)
			}
		default:
			listed = false
		}
	}
	return items
}

// commonDir returns the deepest directory the absolute paths share, their
// parts compared as bytes, or false when they share fewer than one named
// directory or there are none.
func commonDir(paths []string) (string, bool) {
	var common []string
	for number, path := range paths {
		parts := strings.Split(path, "/")
		depth := len(parts) - 1
		if number == 0 {
			common = parts[:depth]
			continue
		}
		common = common[:min(len(common), depth)]
		for i := range common {
			if parts[i] != common[i] {
				common = common[:i]
				break
			}
		}
	}
	if len(paths) == 0 || len(common) < 2 {
		return "", false
	}
	return strings.Join(common, "/"), true
}
