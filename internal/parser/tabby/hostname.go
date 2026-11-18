package tabby

import (
	"net/url"
	"strings"
)

// urlHostname returns the host of a URL in lower case, or "-" when it has
// none or is not a URL net/url parses (a control character, a bad percent
// escape, an IPv6 zone not written as %25).
func urlHostname(text string) string {
	parsed, err := url.Parse(text)
	if err != nil || parsed.Hostname() == "" {
		return "-"
	}
	return strings.ToLower(parsed.Hostname())
}
