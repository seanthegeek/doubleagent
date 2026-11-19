package secrets

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textset"
)

// identifyURLs adds to found the credentials of every scheme://authority URL
// in text, read by RFC 3986: a scheme is a letter then letters, digits, +,
// - and . (section 3.1); the authority follows "//" and ends at the next /,
// ? or # or the end of the URL (section 3.2), and its user information is
// what comes before its last @ (section 3.2.1). User information holding a
// colon (user:password) is a secret whole and its password one on its own;
// without a colon it is a secret only from EverywhereLength bytes, so a
// user name (git@, ci@) stays and a token in its place goes. The value of
// each query or fragment parameter credentialParameters names is one too
// (parameters). Credentials written without a scheme are not identified.
func identifyURLs(text string, found func(form), work textset.Counter) {
	for start := 0; start < len(text); {
		end := start
		for end < len(text) && !isDelimiter(text[end]) {
			end++
		}
		identifyToken(text[start:end], found, work)
		for start = end; start < len(text) && isDelimiter(text[start]); start++ {
		}
	}
	count(work, len(text))
}

// mayHoldURL reports whether text holds a scheme separator, written plain
// or with JSON's escaped slashes, without which it holds no URL.
func mayHoldURL(text string) bool {
	return strings.Contains(text, "://") || strings.Contains(text, `:\/\/`)
}

// URLCredentials returns the credentials identifyURLs finds in text, each
// secret once in the order found, for a test that checks the output
// against them.
func URLCredentials(text string) []string {
	var found []string
	identifyURLs(text, func(secret form) {
		if !slices.Contains(found, secret.secret) {
			found = append(found, secret.secret)
		}
	}, nil)
	return found
}

// isDelimiter reports whether a byte ends a URL inside longer text: white
// space, a double quote and angle brackets, which RFC 3986 appendix C names,
// and a single quote and a backtick, which quote a URL on a command line.
func isDelimiter(char byte) bool {
	return strings.IndexByte(" \t\n\v\f\r\"'<>`", char) >= 0
}

// count adds n bytes read to work, when it counts.
func count(work textset.Counter, n int) {
	if work != nil {
		work.Add("bytes read", n)
	}
}

// identifyToken adds the credentials of each URL in a run of text between
// delimiters. A URL ends where the next one's scheme begins, so each byte
// belongs to one URL and the token is read a fixed number of times. The
// separator "://" may also be written with JSON's escaped slashes (:\/\/).
func identifyToken(token string, found func(form), work textset.Counter) {
	// Punctuation that ends a run of text ends a URL there (RFC 3986
	// appendix C warns of punctuation mistaken for part of a URI), so in
	// "POST https://h/x?key=k: failed" the key's value is k.
	token = strings.TrimRight(token, ".,:;!?")
	var starts [][2]int
	scanned := 0
	for i := 0; i < len(token); i++ {
		separator := separatorAt(token, i)
		if separator == 0 {
			continue
		}
		start, read := schemeStart(token, scanned, i)
		if start >= 0 {
			starts = append(starts, [2]int{start, i + separator})
		}
		count(work, read)
		scanned = i + 1
		i += separator - 1
	}
	for k, start := range starts {
		end := len(token)
		if k+1 < len(starts) {
			end = starts[k+1][0]
		}
		identifyURL(token[start[1]:end], found, work)
	}
	count(work, len(token))
}

// separatorAt returns the length of the separator that starts at byte i of
// token, "://" or `:\/\/`, or 0.
func separatorAt(token string, i int) int {
	switch rest := token[i:]; {
	case strings.HasPrefix(rest, "://"):
		return 3
	case strings.HasPrefix(rest, `:\/\/`):
		return 5
	}
	return 0
}

// schemeStart returns where the scheme before the colon at byte colon of
// token starts, reading back no further than byte from, or -1 when no
// letter begins one there, and the bytes it read.
func schemeStart(token string, from, colon int) (int, int) {
	start := colon
	for start > from && isSchemeByte(token[start-1]) {
		start--
	}
	read := colon - start
	for start < colon && !isLetter(token[start]) {
		start++
	}
	read += start - (colon - read)
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

// identifyURL adds the credentials of one URL from after its separator: its
// user information and its credential parameters. A backslash, which no
// URL holds, ends the authority too, so an escaped slash does.
func identifyURL(rest string, found func(form), work textset.Counter) {
	end := strings.IndexAny(rest, `/?#\`)
	if end < 0 {
		end = len(rest)
	}
	count(work, end)
	authority := rest[:end]
	if atSign := strings.LastIndexByte(authority, '@'); atSign > 0 {
		userInfo := authority[:atSign]
		colon := strings.IndexByte(userInfo, ':')
		if colon >= 0 || len(userInfo) >= EverywhereLength {
			found(form{secret: userInfo})
			// With an empty placeholder the @ goes with the user
			// information, so https://u:p@host becomes https://host.
			found(form{secret: userInfo, after: "@"})
		}
		if colon >= 0 {
			found(form{secret: userInfo[colon+1:]})
		}
	}
	parameters(rest[end:], found, work)
}

// parameters adds the value of each credential parameter of the query and
// the fragment of a URL's text after its authority: the query starts after
// the first ? and ends at #, the fragment starts after #, and each holds
// parameters split at & and ;, a name before the first = and its value
// after. A value is a secret when the name, its percent-escapes decoded,
// is one credentialParameters names; an empty value is none.
func parameters(text string, found func(form), work textset.Counter) {
	start := strings.IndexAny(text, "?#")
	count(work, len(text))
	if start < 0 {
		return
	}
	fragment := text[start] == '#'
	for i := start + 1; i <= len(text); {
		end := i
		for end < len(text) && !endsParameter(text[end], fragment) {
			end++
		}
		name, value, assigned := strings.Cut(text[i:end], "=")
		if assigned && value != "" && isCredentialParameter(decodedName(name)) {
			found(form{secret: value})
		}
		if end < len(text) && text[end] == '#' {
			fragment = true
		}
		i = end + 1
	}
}

// endsParameter reports whether a byte ends a parameter: & or ;, # in a
// query, and a backslash, which no URL holds.
func endsParameter(char byte, fragment bool) bool {
	return char == '&' || char == ';' || char == '\\' || char == '#' && !fragment
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
