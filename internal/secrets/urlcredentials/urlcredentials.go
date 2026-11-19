// Package urlcredentials finds the credentials of URLs inside longer text
// (a command line, JSON, Markdown), where net/url parses one URL, by these
// rules; those citing RFC 3986 follow it, and in doubt they find more:
//
//  1. Text splits into tokens at white space, double quotes and angle
//     brackets (appendix C), and at backticks; nothing is read across them.
//  2. A URL starts at a scheme, a letter then letters, digits, +, - and .
//     (section 3.1), directly followed by "://" or JSON's escaped `:\/\/`.
//  3. A single quote, a ), ] or } that closes no bracket opened earlier in
//     its parameter (rule 6), a comma directly before a scheme, and . , : ;
//     ! ? before one of them or the token's end end the value being read
//     (rule 7) but not the reading: rules 4 to 6 go on.
//  4. The authority follows the separator and ends at the first /, ? or #
//     (section 3.2), a backslash, the next URL's scheme or the token's end.
//  5. The user information is what stands before the authority's last @
//     (section 3.2.1; the last is this package's choice). With a colon it
//     (user:password) is a credential whole, and its password one too,
//     unless it is the colon alone; without, a name (Credential.NameOnly).
//  6. The query runs from the first ? or # after the first URL's authority
//     (sections 3.4 and 3.5) to the token's end. In it a name starts after
//     each &, ;, ?, #, JSON's \u0026 or \U0026, and backslash not before /.
//     Each but ? and # ends a value and a parameter, as does the first #,
//     where the fragment (#access_token=A&token_type=bearer) begins and what
//     a client sends ends. A query may hold ? and a fragment holds no #, so a
//     later # joins a hash route to a fragment (#/cb#access_token=A) or is a
//     token's own; like ?, it ends neither, so no tail of a token shows, and
//     rule 3 counts brackets across it. A name inside a value is part of it.
//  7. A parameter's value, after its first =, is a credential when it is not
//     empty and its name, percent-escapes decoded and ASCII case ignored, is
//     one credentialParameters lists.
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
// work (nil counts nothing): each loop counts the bytes it steps over as it
// steps over them, so each byte is read a fixed number of times however the
// URLs nest. Credentials without a scheme are not found.
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

// urlStart is where one URL begins in a token: its scheme, and its
// authority after the separator.
type urlStart struct {
	scheme, authority int
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

// userInformation adds the credentials of the user information of the
// authority that starts at byte start of text, ending at the first /, ?, #
// or backslash or where text ends, the next URL's scheme or the token's end
// (rules 4 and 5), and returns where it ends.
func userInformation(text string, start int, found func(Credential), work textset.Counter) int {
	end := len(text)
	if stop := strings.IndexAny(text[start:], `/?#\`); stop >= 0 {
		end = start + stop
	}
	// The search for the end read start to end, and the search for the last
	// @ reads the same bytes again: one pass over the span each, however
	// many @ it holds.
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

// IsCredentialName reports whether name, its percent-escapes decoded and
// ASCII case ignored, is a parameter whose value is a credential
// (credentialParameters), for a caller that reads such a name elsewhere,
// such as a JSON member.
func IsCredentialName(name string) bool {
	return isCredentialParameter(decodedName(name))
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
