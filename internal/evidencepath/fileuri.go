package evidencepath

import (
	"net/url"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// FromFileURI returns the path a file: URI (scheme in lower case) names, as
// its machine wrote it, and true; false when uri is not one net/url reads.
// Tabs and line breaks are removed first, as a browser does; the query and
// fragment are dropped; escapes are decoded, a byte not UTF-8 becoming
// U+FFFD. A drive path loses its leading slash: file:///c%3A/x, file:C:/x
// and file://C:/x give c:/x, C:/x and C:/x. A server's host (localhost, with
// or without a port, is no server) stays: file://server/share/x gives
// //server/share/x; a server with no path gives "". An invalid escape or
// host, a control character, an escaped NUL, or a network path with no
// server or a \\?\ or \\.\ prefix gives false. The test table lists every form.
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
	case isServerHost(parsed) && strings.Trim(filePath, "/") == "":
		filePath = ""
	case isServerHost(parsed):
		filePath = "//" + parsed.Host + filePath
	case len(filePath) >= 3 && filePath[0] == '/' && IsASCIILetter(filePath[1]) && filePath[2] == ':':
		filePath = filePath[1:]
	}
	if strings.IndexByte(filePath, 0) >= 0 || isDevicePrefix(filePath) {
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
// empty and not localhost, in any case and with or without a port, the
// forms of this machine.
func isServerHost(parsed *url.URL) bool {
	return parsed.Host != "" && !strings.EqualFold(parsed.Hostname(), "localhost")
}

// isDevicePrefix reports whether a path is a network path whose server is
// empty, "?" or ".": nothing, or the start of a Windows extended-length
// (\\?\) or device (\\.\) path, which no editor's file: URI is known to
// hold, so it is refused rather than guessed at.
func isDevicePrefix(filePath string) bool {
	rest, network := strings.CutPrefix(filePath, "//")
	server, _, _ := strings.Cut(rest, "/")
	return network && (server == "" || server == "?" || server == ".")
}
