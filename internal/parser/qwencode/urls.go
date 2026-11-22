package qwencode

import (
	"net/url"
	"strings"
)

// urlParts are the host name and path of a URL.
type urlParts struct {
	hostname, path string
}

// splitURL returns the host name of a URL, in lower case and without its
// port or the brackets of an IPv6 address, and its path as written (still
// escaped), through net/url. Text net/url refuses (a control character, an
// invalid escape, a malformed host) gives neither, so its row shows "-" for
// the host and repository.
func splitURL(text string) urlParts {
	parsed, err := url.Parse(text)
	if err != nil {
		return urlParts{}
	}
	return urlParts{hostname: strings.ToLower(parsed.Hostname()), path: parsed.EscapedPath()}
}
