package antigravity

import (
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// URIToPath returns the local path of a file: URI, with a Windows drive
// path's leading slash removed ("file:///C:/x" is "C:/x"); any other text
// is returned as it is.
func URIToPath(uri string) string {
	if !strings.HasPrefix(uri, "file:") {
		return uri
	}
	path := URIPath(uri)
	if IsDrivePath(path) {
		path = path[1:]
	}
	return path
}

// URIPath returns the path of a file: URI: the host dropped, the query and
// fragment cut off, tabs and line breaks removed, and percent escapes
// decoded as UTF-8 with replacement.
func URIPath(uri string) string {
	rest := strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(uri)
	rest = rest[strings.IndexByte(rest, ':')+1:]
	if strings.HasPrefix(rest, "//") {
		end := strings.IndexAny(rest[2:], "/?#")
		if end < 0 {
			end = len(rest) - 2
		}
		rest = rest[2+end:]
	}
	rest, _, _ = strings.Cut(rest, "#")
	rest, _, _ = strings.Cut(rest, "?")
	return unquote(rest)
}

// IsDrivePath reports whether a URI's path names a Windows drive: a slash,
// a letter and a colon.
func IsDrivePath(path string) bool {
	return len(path) >= 3 && path[0] == '/' && isASCIILetter(path[1]) && path[2] == ':'
}

// isASCIILetter reports whether char is an ASCII letter.
func isASCIILetter(char byte) bool {
	return ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z')
}

// unquote decodes the percent escapes of text: each run of ASCII characters
// has its %XX escapes turned into bytes and is decoded as UTF-8 with
// replacement; other characters are kept as they are, and a % not followed
// by two hexadecimal digits stays.
func unquote(text string) string {
	if !strings.Contains(text, "%") {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); {
		if text[i] >= 0x80 {
			_, size := utf8.DecodeRuneInString(text[i:])
			out.WriteString(text[i : i+size])
			i += size
			continue
		}
		end := i
		for end < len(text) && text[end] < 0x80 {
			end++
		}
		out.WriteString(textlines.ValidUTF8(percentBytes(text[i:end])))
		i = end
	}
	return out.String()
}

// percentBytes returns ASCII text with each %XX escape as its byte.
func percentBytes(text string) []byte {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '%' && i+2 < len(text) && isHex(text[i+1]) && isHex(text[i+2]) {
			out = append(out, hexValue(text[i+1])<<4|hexValue(text[i+2]))
			i += 2
			continue
		}
		out = append(out, text[i])
	}
	return out
}

// isHex reports whether char is an ASCII hexadecimal digit.
func isHex(char byte) bool {
	return ('0' <= char && char <= '9') || ('a' <= char && char <= 'f') || ('A' <= char && char <= 'F')
}

// hexValue returns the value of a hexadecimal digit.
func hexValue(char byte) byte {
	switch {
	case char <= '9':
		return char - '0'
	case char <= 'F':
		return char - 'A' + 10
	}
	return char - 'a' + 10
}
