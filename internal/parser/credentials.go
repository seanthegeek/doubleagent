package parser

import "strings"

// WithoutCredentials returns a URL with the user name and password it may
// carry before its host (scheme://user:password@host) replaced by
// "redacted", since either can be a token; any other text is returned as
// it is. The text is cut by hand rather than parsed, so a URL net/url
// would refuse is redacted too and the rest of it is shown as written.
func WithoutCredentials(text string) string {
	_, rest, found := strings.Cut(text, "://")
	if !found {
		return text
	}
	start := len(text) - len(rest)
	authority := rest
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		authority = rest[:end]
	}
	userEnd := strings.LastIndexByte(authority, '@')
	if userEnd < 0 {
		return text
	}
	return text[:start] + "redacted" + text[start+userEnd:]
}
