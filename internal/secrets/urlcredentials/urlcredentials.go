// Package urlcredentials finds the credentials of the URLs in a text. Where
// net/url parses one URL and refuses a bad percent-escape, it finds URLs
// inside longer text (a command line, JSON, Markdown) by this grammar. The
// parts that cite RFC 3986 follow it; the rest are this package's own:
//
//  1. Text splits into tokens at white space, double quotes and angle
//     brackets (appendix C), and at single quotes and backticks.
//  2. A URL starts at a scheme, a letter then letters, digits, +, - and .
//     (section 3.1), directly followed by "://" or JSON's escaped `:\/\/`.
//  3. A URL ends where its token ends, at a ), ] or } that closes no
//     bracket opened inside the URL, or at a comma directly before a scheme;
//     a URL that starts before it ends is inside it. The punctuation . , : ;
//     ! ? at its end belongs to the sentence, not to it.
//  4. The authority follows the separator and ends at the first /, ? or #
//     or where the URL ends (section 3.2), and at a backslash or the scheme
//     of a URL inside it.
//  5. The user information is what stands before the authority's last @
//     (section 3.2.1). With a colon (user:password) it is a credential
//     whole, and its password one too, unless it is the colon alone; without
//     one it is a user name alone (Credential.NameOnly).
//  6. The query starts at the first ? after the authority of a URL that is
//     inside no other, and the fragment at #; both run on to where that URL
//     ends, over the URLs inside it (a redirect target), which have no
//     query of their own.
//  7. Parameters split at & and ;, at # in the query, and at JSON's escaped
//     \u0026 or \U0026; \/ (an escaped slash) is part of a value and any
//     other backslash ends one.
//  8. A parameter's value, after its first =, is a credential when it is not
//     empty and its name, its percent-escapes decoded and ASCII case
//     ignored, is one credentialParameters lists.
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

// MayHold reports whether text holds a scheme separator, written plain or
// with JSON's escaped slashes, without which it holds no URL.
func MayHold(text string) bool {
	return strings.Contains(text, "://") || strings.Contains(text, `:\/\/`)
}

// Find calls found with the credentials of every URL in text, in the order
// of the grammar of the package comment, counting the bytes it reads in
// work (nil counts nothing). Each byte is read a fixed number of times,
// however the URLs nest; credentials without a scheme are not found.
func Find(text string, found func(Credential), work textset.Counter) {
	for start := 0; start < len(text); {
		end := start
		for end < len(text) && !isDelimiter(text[end]) {
			end++
		}
		identifyToken(text[:end], start, found, work)
		for start = end; start < len(text) && isDelimiter(text[start]); start++ {
		}
	}
	count(work, len(text))
}

// isDelimiter reports whether a byte ends a token (rule 1): white space, a
// double quote and angle brackets, which RFC 3986 appendix C names, and a
// single quote and a backtick, which quote a URL on a command line.
func isDelimiter(char byte) bool {
	return strings.IndexByte(" \t\n\v\f\r\"'<>`", char) >= 0
}

// count adds n bytes read to work, when it counts.
func count(work textset.Counter, n int) {
	if work != nil {
		work.Add("bytes read", n)
	}
}

// urlStart is where one URL begins in a token: its scheme, and its
// authority after the separator.
type urlStart struct {
	scheme, authority int
}

// identifyToken adds the credentials of the URLs in text[from:], a token
// that ends where text ends (rule 1): it finds every URL's start (rule 2),
// then reads each outer URL to its end (rule 3) with the URLs inside it.
func identifyToken(text string, from int, found func(Credential), work textset.Counter) {
	starts := urlStarts(text, from, work)
	for k := 0; k < len(starts); {
		end, next := urlEnd(text, starts, k, work)
		identifyURL(text[:end], starts[k:next], found, work)
		k = next
		for k < len(starts) && starts[k].scheme < end {
			k++
		}
	}
	count(work, len(text)-from)
}

// urlStarts returns the start of every URL in text[from:] (rule 2). A
// scheme is read back no further than the previous separator, so each byte
// is read once.
func urlStarts(text string, from int, work textset.Counter) []urlStart {
	var starts []urlStart
	scanned := from
	for i := from; i < len(text); i++ {
		separator := separatorAt(text, i)
		if separator == 0 {
			continue
		}
		start, read := schemeStart(text, scanned, i)
		if start >= 0 {
			starts = append(starts, urlStart{start, i + separator})
		}
		count(work, read)
		scanned = i + separator
		i += separator - 1
	}
	return starts
}

// urlEnd returns where the URL that starts at starts[k] ends (rule 3), and
// the index in starts of the first URL that starts at or after that end,
// or of the first after a comma that ends it; those between are inside it.
func urlEnd(text string, starts []urlStart, k int, work textset.Counter) (int, int) {
	var open [3]int
	next := k + 1
	end := starts[k].scheme
	for ; end < len(text); end++ {
		if next < len(starts) && end == starts[next].scheme-1 && text[end] == ',' {
			break
		}
		if next < len(starts) && end == starts[next].scheme {
			next++
		}
		if bracket := strings.IndexByte("([{)]}", text[end]); bracket >= 3 {
			if open[bracket-3] == 0 {
				break
			}
			open[bracket-3]--
		} else if bracket >= 0 {
			open[bracket]++
		}
	}
	count(work, end-starts[k].scheme)
	trimmed := strings.TrimRight(text[starts[k].scheme:end], ".,:;!?")
	return starts[k].scheme + len(trimmed), next
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

// isSchemeByte reports whether a byte may stand in a scheme.
func isSchemeByte(char byte) bool {
	return isLetter(char) || '0' <= char && char <= '9' || char == '+' || char == '-' || char == '.'
}

// isLetter reports whether a byte is an ASCII letter.
func isLetter(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z'
}

// identifyURL adds the credentials of an outer URL that ends where text
// ends, starts[0], and of the URLs inside it, the rest of starts: the user
// information of each (rules 4 and 5) and the outer URL's parameters
// (rules 6 to 8).
func identifyURL(text string, starts []urlStart, found func(Credential), work textset.Counter) {
	authorityEnd := len(text)
	for k := len(starts) - 1; k >= 0; k-- {
		limit := len(text)
		if k+1 < len(starts) {
			limit = starts[k+1].scheme
		}
		authorityEnd = userInformation(text[:limit], starts[k].authority, found, work)
	}
	parameters(text, authorityEnd, found, work)
}

// userInformation adds the credentials of the user information of the
// authority that starts at byte start of text, ending at the first /, ?, #
// or backslash or where text ends (rules 4 and 5), and returns where it
// ends.
func userInformation(text string, start int, found func(Credential), work textset.Counter) int {
	end := len(text)
	if stop := strings.IndexAny(text[start:], `/?#\`); stop >= 0 {
		end = start + stop
	}
	count(work, end-start)
	atSign := strings.LastIndexByte(text[start:end], '@')
	if atSign <= 0 {
		return end
	}
	userInfo := text[start : start+atSign]
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

// parameters adds the value of each credential parameter of the query and
// fragment of a URL that ends where text ends, read from byte from, its
// authority's end (rules 6 to 8).
func parameters(text string, from int, found func(Credential), work textset.Counter) {
	count(work, len(text)-from)
	start := strings.IndexAny(text[from:], "?#")
	if start < 0 {
		return
	}
	start += from
	fragment := text[start] == '#'
	for i := start + 1; i <= len(text); {
		end, separator := parameterEnd(text, i, fragment)
		name, value, assigned := strings.Cut(text[i:end], "=")
		if assigned && value != "" && isCredentialParameter(decodedName(name)) {
			found(Credential{Start: end - len(value), End: end})
		}
		if end < len(text) && text[end] == '#' {
			fragment = true
		}
		i = end + separator
	}
}

// parameterEnd returns where the parameter that starts at byte i of text
// ends and the length of what ends it (rule 7): & or ;, # in a query,
// \u0026 or \U0026, or a backslash not before a slash; 1 at the end of the
// text.
func parameterEnd(text string, i int, fragment bool) (int, int) {
	for end := i; end < len(text); end++ {
		switch char := text[end]; {
		case char == '&' || char == ';' || char == '#' && !fragment:
			return end, 1
		case char != '\\':
		case strings.HasPrefix(text[end:], `\/`):
			end++
		case strings.HasPrefix(text[end:], `\u0026`) || strings.HasPrefix(text[end:], `\U0026`):
			return end, 6
		default:
			return end, 1
		}
	}
	return len(text), 1
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

// isCredentialParameter reports whether a lowered parameter name is one
// credentialParameters names.
func isCredentialParameter(name string) bool {
	for _, parameter := range credentialParameters() {
		if len(parameter.name) == len(name) && decodedName(parameter.name) == name {
			return true
		}
	}
	return false
}
