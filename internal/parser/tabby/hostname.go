package tabby

import "strings"

// schemeCharacters are the characters a URL scheme is made of.
const schemeCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+-."

// urlHostname returns the host of a URL as Python's urlparse(text).hostname
// gives it, lower-cased, or "-" when it has none: leading control
// characters and spaces are dropped and tabs and line breaks removed, a
// scheme is skipped, and the host is the part of "//user@host:port"
// between the user and the port, or inside the brackets of an IPv6
// address, an IPv6 zone keeping its case.
func urlHostname(text string) string {
	text = strings.TrimLeftFunc(text, func(char rune) bool { return char <= ' ' })
	text = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(text)
	if colon := strings.IndexByte(text, ':'); colon > 0 && isLetter(text[0]) &&
		strings.Trim(text[:colon], schemeCharacters) == "" {
		text = text[colon+1:]
	}
	netloc, found := strings.CutPrefix(text, "//")
	if !found {
		return "-"
	}
	if end := strings.IndexAny(netloc, "/?#"); end >= 0 {
		netloc = netloc[:end]
	}
	hostinfo := netloc[strings.LastIndexByte(netloc, '@')+1:]
	var hostname string
	if _, bracketed, open := strings.Cut(hostinfo, "["); open {
		hostname, _, _ = strings.Cut(bracketed, "]")
	} else {
		hostname, _, _ = strings.Cut(hostinfo, ":")
	}
	if hostname == "" {
		return "-"
	}
	host, zone, percent := strings.Cut(hostname, "%")
	if percent {
		return strings.ToLower(host) + "%" + zone
	}
	return strings.ToLower(host)
}

// isLetter reports whether a byte is an ASCII letter.
func isLetter(octet byte) bool {
	return octet >= 'a' && octet <= 'z' || octet >= 'A' && octet <= 'Z'
}
