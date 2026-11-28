package urlcredentials

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textset"
)

// Credential is one credential a URL holds in a text: the bytes
// text[Start:End], and After, the number of bytes after it that an empty
// placeholder takes away with it (the @ after user information). NameOnly
// marks user information without a colon, which may be a user name only.
type Credential struct {
	Start, End, After int
	NameOnly          bool
}

// ReplacedEnd returns where the bytes replaced for the credential end:
// after the bytes an empty placeholder takes with it, or at its end.
func (c Credential) ReplacedEnd(emptyPlaceholder bool) int {
	if emptyPlaceholder {
		return c.End + c.After
	}
	return c.End
}

// FirstSeparator returns where the first scheme separator of text, "://" or
// JSON's `:\/\/`, starts and its length, or -1 and 0 when text holds none.
func FirstSeparator(text string) (int, int) {
	plain, escaped := strings.Index(text, "://"), strings.Index(text, `:\/\/`)
	switch {
	case escaped >= 0 && (plain < 0 || escaped < plain):
		return escaped, len(`:\/\/`)
	case plain >= 0:
		return plain, len("://")
	}
	return -1, 0
}

// MayHold reports whether text holds a scheme separator, written plain or
// with JSON's escaped slashes, without which it holds no URL.
func MayHold(text string) bool {
	separator, _ := FirstSeparator(text)
	return separator >= 0
}

// isDelimiter reports whether a byte ends a token (rule 1): white space, a
// double quote and angle brackets, which RFC 3986 appendix C names, and a
// backtick, which quotes a command in Markdown and on a command line.
func isDelimiter(char byte) bool {
	return strings.IndexByte(" \t\n\v\f\r\"<>`", char) >= 0
}

// count adds n bytes read to work, when it counts.
func count(work textset.Counter, n int) {
	if work != nil {
		work.Add("bytes read", n)
	}
}

// separatorAt returns the length of the separator that starts at byte i of
// text, "://" or `:\/\/`, or 0.
func separatorAt(text string, i int) int {
	switch rest := text[i:]; {
	case strings.HasPrefix(rest, "://"):
		return 3
	case strings.HasPrefix(rest, `:\/\/`):
		return 5
	}
	return 0
}

// isLetter reports whether a byte is an ASCII letter.
func isLetter(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z'
}

// isSchemeByte reports whether a byte may stand in a scheme.
func isSchemeByte(char byte) bool {
	return isLetter(char) || '0' <= char && char <= '9' || char == '+' || char == '-' || char == '.'
}

// schemeStart returns where the scheme before the colon at byte colon of
// text starts, reading back no further than byte from, or -1 when no
// letter begins one there, and the bytes it read.
func schemeStart(text string, from, colon int) (int, int) {
	start := colon
	for start > from && isSchemeByte(text[start-1]) {
		start--
	}
	read := colon - start
	for start < colon && !isLetter(text[start]) {
		start++
	}
	if start == colon {
		return -1, read
	}
	return start, read
}

// urlStart is where one URL begins in a token: its scheme, and its
// authority after the separator.
type urlStart struct {
	scheme, authority int
}

// urlStarts returns the start of every URL in text[from:] (rule 2). A
// scheme is read back no further than the previous separator, so each byte
// is read once.
func urlStarts(text string, from int, work textset.Counter) []urlStart {
	var starts []urlStart
	scanned := from
	for i := from; i < len(text); i++ {
		count(work, 1)
		separator := separatorAt(text, i)
		if separator == 0 {
			continue
		}
		start, read := schemeStart(text, scanned, i)
		if start >= 0 {
			starts = append(starts, urlStart{start, i + separator})
		}
		count(work, read+separator-1)
		scanned = i + separator
		i += separator - 1
	}
	return starts
}

// userInformation adds the credentials of the user information of the
// authority that starts at byte start of text, ending at the first /, ?, #
// or backslash or where text ends, the next URL's scheme or the token's end
// (rules 4 and 5), and returns where it ends.
func userInformation(text string, start int, found func(Credential), work textset.Counter) int {
	end := len(text)
	if stop := strings.IndexAny(text[start:], `/?#\`); stop >= 0 {
		end = start + stop
	}
	count(work, 2*(end-start))
	atSign := strings.LastIndexByte(text[start:end], '@')
	if atSign <= 0 {
		return end
	}
	userInfo := text[start : start+atSign]
	count(work, atSign)
	colon := strings.IndexByte(userInfo, ':')
	if userInfo != ":" {
		// With an empty placeholder the @ goes with the user information,
		// so https://u:p@host becomes https://host.
		found(Credential{Start: start, End: start + atSign, After: 1, NameOnly: colon < 0})
	}
	if colon >= 0 && colon+1 < atSign {
		found(Credential{Start: start + colon + 1, End: start + atSign})
	}
	return end
}

// identifyToken adds the credentials of the URLs in text[from:], a token
// that ends where text ends (rule 1): it finds every URL's start (rule 2),
// the user information of each (rules 4 and 5), then the parameters of
// the query that runs from the first URL's authority (rules 3, 6 and 7).
func identifyToken(text string, from int, found func(Credential), work textset.Counter) {
	starts := urlStarts(text, from, work)
	if len(starts) > 0 {
		firstAuthorityEnd := 0
		for k := range starts {
			limit := len(text)
			if k+1 < len(starts) {
				limit = starts[k+1].scheme
			}
			end := userInformation(text[:limit], starts[k].authority, found, work)
			if k == 0 {
				firstAuthorityEnd = end
			}
		}
		parameters(text, starts, firstAuthorityEnd, found, work)
	}
}

// Find calls found with the credentials of every URL in text, in the order
// of the grammar of the package comment, counting the bytes it reads in
// work (nil counts nothing); each byte is read a fixed number of times
// however the URLs nest. Credentials without a scheme are not found.
func Find(text string, found func(Credential), work textset.Counter) {
	for start := 0; start < len(text); {
		end := start
		for end < len(text) && !isDelimiter(text[end]) {
			end++
			count(work, 1)
		}
		identifyToken(text[:end], start, found, work)
		for start = end; start < len(text) && isDelimiter(text[start]); start++ {
			count(work, 1)
		}
	}
}

// TokenStart returns where the token (rule 1) that text[:end] ends in
// starts: just after the last delimiter before end, or 0. It reads back
// from end no further than that delimiter.
func TokenStart(text string, end int) int {
	start := end
	for start > 0 && !isDelimiter(text[start-1]) {
		start--
	}
	return start
}

// TokenEnd returns where the token (rule 1) that holds byte start of text
// ends: at the first delimiter at or after start, or at the end of text.
func TokenEnd(text string, start int) int {
	end := start
	for end < len(text) && !isDelimiter(text[end]) {
		end++
	}
	return end
}

// Schemes returns where each URL of text (rule 2) begins and its authority
// starts, [scheme, authority), in order: the bytes of its scheme and its
// separator.
func Schemes(text string) [][2]int {
	var spans [][2]int
	for start := 0; start < len(text); {
		end := TokenEnd(text, start)
		for _, each := range urlStarts(text[:end], start, nil) {
			spans = append(spans, [2]int{each.scheme, each.authority})
		}
		start = end
		for start < len(text) && isDelimiter(text[start]) {
			start++
		}
	}
	return spans
}

// hexValue returns the value of the hex digit at byte i of text, -1 when
// there is none.
func hexValue(text string, i int) int {
	if i >= len(text) {
		return -1
	}
	switch char := text[i]; {
	case '0' <= char && char <= '9':
		return int(char - '0')
	case 'a' <= char && char <= 'f':
		return int(char-'a') + 10
	case 'A' <= char && char <= 'F':
		return int(char-'A') + 10
	}
	return -1
}

// decodedName returns a parameter's name with each %XX escape decoded and
// ASCII capitals lowered; an escape that is not two hex digits is kept.
func decodedName(name string) string {
	var out strings.Builder
	for i := 0; i < len(name); i++ {
		char := name[i]
		if high, low := hexValue(name, i+1), hexValue(name, i+2); char == '%' && high >= 0 && low >= 0 {
			char = byte(high<<4 | low)
			i += 2
		}
		if 'A' <= char && char <= 'Z' {
			char += 'a' - 'A'
		}
		out.WriteByte(char)
	}
	return out.String()
}

// IsCredentialName reports whether name, its percent-escapes decoded and
// ASCII case ignored, is a parameter whose value is a credential
// (isCredentialParameter), for a caller that reads such a name elsewhere,
// such as a JSON member.
func IsCredentialName(name string) bool {
	return isCredentialParameter(decodedName(name))
}
