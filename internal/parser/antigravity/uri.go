package antigravity

import "github.com/seanthegeek/doubleagent/internal/evidencepath"

// uriPath returns the path a file: URI names, read by
// evidencepath.FromFileURI (a drive path loses its leading slash:
// "file:///C:/x" is "C:/x"). Antigravity also writes plain paths in some
// of the same fields, so any other text, and a file: URI that net/url
// cannot read, is returned as it is.
func uriPath(uri string) string {
	if path, ok := evidencepath.FromFileURI(uri); ok {
		return path
	}
	return uri
}
