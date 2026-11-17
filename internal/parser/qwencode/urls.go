package qwencode

import (
	"strings"
)

// urlSchemeCharacters are the characters a URL scheme may hold after its
// first letter.
const urlSchemeCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+-."

// urlParts are the host name and path of a URL, split as Python's
// urllib.parse.urlsplit splits one.
type urlParts struct {
	hostname, path string
}

// splitURL returns a URL's host name and path. The URL loses its leading
// control characters and spaces and every tab, CR and LF; a scheme is
// letters, digits, "+", "-" and "." before the first ":", opening with an
// ASCII letter; the authority follows "//" up to the first "/", "?" or "#";
// the path runs to the query or fragment. The host name is the authority
// after its last "@", without a port or the brackets of an IPv6 address,
// lower-cased up to any zone after "%".
func splitURL(url string) urlParts {
	url = strings.TrimLeft(url, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f"+
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f ")
	url = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(url)
	if colon := strings.IndexByte(url, ':'); colon > 0 && isASCIILetter(url[0]) &&
		strings.Trim(url[:colon], urlSchemeCharacters) == "" {
		url = url[colon+1:]
	}
	authority := ""
	if rest, ok := strings.CutPrefix(url, "//"); ok {
		end := len(rest)
		if found := strings.IndexAny(rest, "/?#"); found >= 0 {
			end = found
		}
		authority, url = rest[:end], rest[end:]
	}
	url, _, _ = strings.Cut(url, "#")
	url, _, _ = strings.Cut(url, "?")
	return urlParts{hostname: hostName(authority), path: url}
}

// isASCIILetter reports whether a byte is an ASCII letter.
func isASCIILetter(character byte) bool {
	return 'a' <= character && character <= 'z' || 'A' <= character && character <= 'Z'
}

// hostName returns the host name of a URL's authority, "" for none.
func hostName(authority string) string {
	info := authority[strings.LastIndexByte(authority, '@')+1:]
	var host string
	if _, bracketed, ok := strings.Cut(info, "["); ok {
		host, _, _ = strings.Cut(bracketed, "]")
	} else {
		host, _, _ = strings.Cut(info, ":")
	}
	host, zone, hasZone := strings.Cut(host, "%")
	host = strings.ToLower(host)
	if hasZone {
		host += "%" + zone
	}
	return host
}
