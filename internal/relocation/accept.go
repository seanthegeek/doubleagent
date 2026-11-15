package relocation

import "strings"

// AcceptValue keeps value when it is a plain absolute path: it starts with
// `/` and holds no `$`, `%`, backquote, quote, control character or `..`
// segment. The kept path loses its trailing slashes and gains root in front,
// since in image mode it names a path inside the image. Otherwise accepted is
// empty and reason names the kind of text, never quoting it, in the order the
// sh collector tests: newline, control character, `$`, `%`, backquote, quote,
// `..` segment, then what a path that is not absolute starts with.
func AcceptValue(value, root string) (accepted, reason string) {
	switch {
	case strings.Contains(value, "\n"):
		return "", "holds a newline"
	case hasControl(value):
		return "", "holds a control character"
	case strings.Contains(value, "$"):
		return "", dollarReason(value)
	case strings.Contains(value, "%"):
		return "", percentReason(value)
	case strings.Contains(value, "`"):
		return "", "holds a backquote"
	case strings.ContainsAny(value, `"'`):
		return "", "holds a quote"
	case strings.HasPrefix(value, "/"):
		return absoluteValue(value, root)
	}
	return "", relativeReason(value)
}

// hasControl reports a control byte other than NUL in text, as the sh
// collector's CONTROL_CHARACTERS set lists them: 0x01 to 0x1f and 0x7f.
func hasControl(text string) bool {
	for i := 0; i < len(text); i++ {
		if (text[i] >= 0x01 && text[i] <= 0x1f) || text[i] == 0x7f {
			return true
		}
	}
	return false
}

// dollarReason names the variable the first `$` of value starts, braces
// allowed, or says it starts none (a `$$`, a `$1`, a `$-`).
func dollarReason(value string) string {
	_, name, _ := strings.Cut(value, "$")
	name = strings.TrimPrefix(name, "{")
	name = name[:wordLength(name)]
	if name == "" || isDigit(name[0]) {
		return "holds a $"
	}
	return "references $" + name
}

// percentReason names the `%NAME%` reference the first `%` of value starts,
// or says it starts none (a `%%`).
func percentReason(value string) string {
	_, after, _ := strings.Cut(value, "%")
	if after == "" || !isNameStart(after[0]) {
		return "holds a %"
	}
	name, _, closed := strings.Cut(after, "%")
	if !closed || wordLength(name) != len(name) {
		return "holds a %"
	}
	return "references %" + name + "%"
}

// absoluteValue refuses a `..` segment, drops trailing slashes from the rest
// and puts root in front; the root path `/` becomes root itself.
func absoluteValue(value, root string) (accepted, reason string) {
	if strings.Contains(value+"/", "/../") {
		return "", "holds a .. segment"
	}
	for value != "/" && strings.HasSuffix(value, "/") {
		value = value[:len(value)-1]
	}
	if value == "/" && root != "" {
		return root, ""
	}
	return root + value, ""
}

// relativeReason classifies a value not starting with `/`: `uri` when the
// text before its first colon is a scheme of at least two characters (the sh
// test also takes an empty one), `relative` when it starts with a letter,
// digit, dot, underscore or hyphen, else `not an absolute path`.
func relativeReason(value string) string {
	reason := "not an absolute path"
	if value != "" && (isWordByte(value[0]) || value[0] == '.' || value[0] == '-') {
		reason = "relative"
	}
	scheme, _, hasColon := strings.Cut(value, ":")
	if !hasColon || len(scheme) == 1 || (scheme != "" && !isLetter(scheme[0])) {
		return reason
	}
	for i := 0; i < len(scheme); i++ {
		if !isLetter(scheme[i]) && !isDigit(scheme[i]) && strings.IndexByte("+.-", scheme[i]) < 0 {
			return reason
		}
	}
	return "uri"
}

// wordLength returns the length of the run of letters, digits and
// underscores that text starts with.
func wordLength(text string) int {
	for i := 0; i < len(text); i++ {
		if !isWordByte(text[i]) {
			return i
		}
	}
	return len(text)
}

// isWordByte reports an ASCII letter, digit or underscore, the characters a
// shell variable name holds.
func isWordByte(char byte) bool { return isNameStart(char) || isDigit(char) }

// isNameStart reports an ASCII letter or underscore.
func isNameStart(char byte) bool { return isLetter(char) || char == '_' }

// isLetter reports an ASCII letter; the sh collector matches under LC_ALL=C.
func isLetter(char byte) bool { return (char|0x20) >= 'a' && (char|0x20) <= 'z' }

// isDigit reports an ASCII digit.
func isDigit(char byte) bool { return char >= '0' && char <= '9' }
