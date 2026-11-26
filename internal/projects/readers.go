package projects

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
)

// The readers below are `grep -o` and `sed` style line matchers, not
// parsers: each pattern is a POSIX extended expression,
// matched leftmost-longest as grep and sed match, and applied to one line at
// a time, so a value never spans a newline. The keys are literal names fixed
// in this package, quoted before they reach a pattern.

// groupValues returns the first parenthesised group of every match of
// pattern in line: the value a `grep -o` match keeps once sed strips the
// fixed text around it.
func groupValues(pattern *regexp.Regexp, line []byte) []string {
	var values []string
	for _, match := range pattern.FindAllSubmatch(line, -1) {
		values = append(values, string(match[1]))
	}
	return values
}

// jsonValueMatcher matches the value of each `"KEY": "..."` in a line.
func jsonValueMatcher(key string) func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`"` + regexp.QuoteMeta(key) + `": *"([^"]*)"`)
	return func(line []byte) []string { return groupValues(pattern, line) }
}

// jsonPathKeyMatcher matches each quoted key starting with `/`
// followed by a colon and next, a bracket expression or a literal byte.
// Under the driveJSON rule a key starting with a drive letter and a colon
// counts too.
func jsonPathKeyMatcher(next string, rules pathRules) func(line []byte) []string {
	key := `/[^"]*`
	if rules.driveJSON {
		key = `(/|[A-Za-z]:)[^"]*`
	}
	pattern := regexp.MustCompilePOSIX(`"(` + key + `)": *` + next)
	return func(line []byte) []string { return groupValues(pattern, line) }
}

// absStringMatcher matches every quoted absolute path or file:// URI
// in a line, turned into a path by localPath. Under the driveJSON rule a
// drive path written with an escaped backslash counts too, made a slashed
// path by drivePathText rather than localPath, which decodes no drive path.
func absStringMatcher(rules pathRules) func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`"((file://)?/[^"]*)"`)
	if rules.driveJSON {
		pattern = regexp.MustCompilePOSIX(`"((file://)?/[^"]*|[A-Za-z]:\\\\[^"]*)"`)
	}
	return func(line []byte) []string {
		values := groupValues(pattern, line)
		for i, value := range values {
			if evidencepath.IsDrivePath(value) {
				values[i] = drivePathText(value)
			} else {
				values[i] = localPath(value)
			}
		}
		return values
	}
}

// drivePathText returns a drive path read from JSON text with `\\` and `\/`
// undone and every backslash a slash, so commonDir splits it into its
// segments.
func drivePathText(value string) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, `\\`, `\`), `\/`, "/")
	return strings.ReplaceAll(value, `\`, "/")
}

// localPath turns an absStringMatcher value into a path: a file:// URL loses
// its scheme and is decoded (decodeURLPath), and a drive URL (`file:///C:/x`,
// `file:///C%3A/x`) keeps only its path below the drive.
func localPath(value string) string {
	rest, isURL := strings.CutPrefix(value, "file://")
	if !isURL {
		return value
	}
	rest = decodeURLPath(rest)
	if len(rest) >= 4 && rest[0] == '/' && isLetter(rest[1]) && rest[2] == ':' && rest[3] == '/' {
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

// isLetter is the C locale's [A-Za-z].
func isLetter(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

// isQuote reports whether char is a double or a single quote.
func isQuote(char byte) bool { return char == '"' || char == '\'' }

// yamlValueMatcher matches a line starting with `KEY:` gives the rest
// after the blanks, less one leading and then one trailing quote of either
// kind, the two not required to pair.
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

// json5ValueMatcher matches the string value of KEY, the key quoted
// with either quote or bare and not preceded by a name character, the value
// in either quote. The key prefix is cut at the last place the
// key, colon and opening quote occur in the match, so a value that repeats
// that text keeps only what follows it.
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
				value = value[:len(value)-1]
			}
			values = append(values, value)
		}
		return values
	}
}

// tomlFileURLMatcher matches the path of a `KEY = "file://..."`
// line, either quote, decoded by decodeURLPath.
func tomlFileURLMatcher(key string) func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`^ *` + regexp.QuoteMeta(key) + ` *= *["']file://([^"']*)["']`)
	return func(line []byte) []string { return decodeURLPaths(groupValues(pattern, line)) }
}

// sessionCwdMatcher matches each `"cwd":"..."`, no blank
// allowed after the colon.
func sessionCwdMatcher() func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`"cwd":"([^"]*)"`)
	return func(line []byte) []string { return groupValues(pattern, line) }
}

// workspaceFolderMatcher matches the path of each `"folder": "file://..."`
// in a VS Code workspace.json, decoded by decodeURLPath.
func workspaceFolderMatcher() func(line []byte) []string {
	pattern := regexp.MustCompilePOSIX(`"folder": *"file://([^"]*)"`)
	return func(line []byte) []string { return decodeURLPaths(groupValues(pattern, line)) }
}

// additionalDirectoryMatcher is Agent Zero's two-grep pipeline: the quoted
// absolute paths inside each `"acp_additional_directories": [...]` list
// that closes on the same line.
func additionalDirectoryMatcher() func(line []byte) []string {
	list := regexp.MustCompilePOSIX(`"acp_additional_directories": *\[[^]]*\]`)
	item := regexp.MustCompilePOSIX(`"(/[^"]*)"`)
	return func(line []byte) []string {
		var values []string
		for _, match := range list.FindAll(line, -1) {
			values = append(values, groupValues(item, match)...)
		}
		return values
	}
}

// decodeURLPaths replaces each value by its decodeURLPath.
func decodeURLPaths(values []string) []string {
	for i, value := range values {
		values[i] = decodeURLPath(value)
	}
	return values
}

// gooseWorkingDirs reads a Goose project file's lines: the items of a `workingDirs:` block list, trailing blanks and
// carriage returns removed, one enclosing pair of quotes removed, an empty
// item skipped; any other line ends the list.
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
