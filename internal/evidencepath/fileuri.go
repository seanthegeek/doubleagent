package evidencepath

import (
	"net/url"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// FromFileURI returns the path a file: URI names, as the evidence's machine
// wrote it, and whether uri is a file: URI (scheme in lower case) that
// net/url can read. Tabs and line breaks are removed first, as a browser
// does; the query and fragment are dropped; escapes are decoded, a byte that
// is not UTF-8 becoming U+FFFD. A drive path loses its leading slash:
// file:///c%3A/x, file:C:/x and file://C:/x give c:/x, C:/x and C:/x. A
// server's host (not localhost) stays: file://server/share/x gives
// //server/share/x, and a server with no path names no file, "". An invalid
// escape, a malformed host, another control character or an escaped NUL
// refuses the URI: false. The test table lists every form.
func FromFileURI(uri string) (string, bool) {
	if !strings.HasPrefix(uri, "file:") {
		return "", false
	}
	parsed, err := url.Parse(strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(uri))
	if err != nil {
		return "", false
	}
	filePath := parsed.Path
	if parsed.Opaque != "" {
		if filePath, err = url.PathUnescape(parsed.Opaque); err != nil {
			return "", false
		}
	}
	switch {
	case isDriveHost(parsed.Host):
		filePath = parsed.Host + filePath
	case isServerHost(parsed.Host) && strings.Trim(filePath, "/") == "":
		filePath = ""
	case isServerHost(parsed.Host):
		filePath = "//" + parsed.Host + filePath
	case len(filePath) >= 3 && filePath[0] == '/' && IsASCIILetter(filePath[1]) && filePath[2] == ':':
		filePath = filePath[1:]
	}
	if strings.IndexByte(filePath, 0) >= 0 {
		return "", false
	}
	return textlines.ValidUTF8([]byte(filePath)), true
}

// isDriveHost reports whether a URI's host is a drive letter and a colon
// with no port: file://C:/x is a common misspelling of file:///C:/x.
func isDriveHost(host string) bool {
	return len(host) == 2 && IsASCIILetter(host[0]) && host[1] == ':'
}

// isServerHost reports whether a URI's host names another machine: not
// empty and not localhost, in any case, the two forms of this machine.
func isServerHost(host string) bool {
	return host != "" && !strings.EqualFold(host, "localhost")
}
