package secrets

import "strings"

// withheldURLs returns text with the credentials of every URL in it
// replaced by placeholder: the user information between "scheme://" and
// the host, a user name alone included (userInfoEnd); user:password@
// written without a scheme (schemelessToken); and the value of a query or
// fragment parameter credentialName names (withheldParameters). A URL ends
// at the delimiters of a URL in text that RFC 3986 appendix C names (white
// space, a quote, an angle bracket, a backtick, a backslash). The text is
// cut by hand, not parsed, so a URL net/url refuses is handled too, and the
// rest of each URL is kept as written.
func withheldURLs(text, placeholder string) string {
	var out strings.Builder
	for start := 0; start < len(text); {
		end := start
		for end < len(text) && !isDelimiter(text[end]) {
			end++
		}
		out.WriteString(withheldToken(text[start:end], placeholder))
		for start = end; start < len(text) && isDelimiter(text[start]); start++ {
			out.WriteByte(text[start])
		}
	}
	return out.String()
}

// isDelimiter reports whether a byte ends a URL inside longer text.
func isDelimiter(char byte) bool {
	return strings.IndexByte(" \t\n\v\f\r\"'<>`\\", char) >= 0
}

// withheldToken returns one run of text between delimiters with its
// credentials replaced: the user information after each "://" in it and
// the credential parameters after the first, or, when it has no "://", a
// user:password@ written without a scheme.
func withheldToken(token, placeholder string) string {
	first := strings.Index(token, "://")
	if first < 0 {
		return schemelessToken(token, placeholder)
	}
	var out strings.Builder
	rest := token
	for {
		before, after, found := strings.Cut(rest, "://")
		if !found {
			out.WriteString(rest)
			break
		}
		out.WriteString(before + "://")
		if end := userInfoEnd(after); end >= 0 {
			out.WriteString(withAtSign(placeholder))
			after = after[end+1:]
		}
		rest = after
	}
	withheld := out.String()
	return withheld[:first] + withheldParameters(withheld[first:], placeholder)
}

// withAtSign returns the placeholder with the @ that ends user information,
// or nothing for an empty placeholder.
func withAtSign(placeholder string) string {
	if placeholder == "" {
		return ""
	}
	return placeholder + "@"
}

// userInfoEnd returns the position of the @ that ends the user information
// at the start of rest, the text after "://", or -1. An @ before the first
// /, ? or # ends it (the last such @). Else, so a password holding /, ? or
// # is caught, an @ after that first one ends it when a colon comes before
// that first one and is not a port (isPort): the first such @, or the last
// of the run from it to the next /, ? or #. So host/a:b/c@d and
// host.example:8080/@x keep their paths, while devbox:8080/@x loses host
// and path rather than show a password. A password holding an @ followed
// later by /, ? or # is cut at that @: no rule tells it from a path.
func userInfoEnd(rest string) int {
	if strings.HasPrefix(rest, "[") {
		return -1
	}
	cut := strings.IndexAny(rest, "/?#")
	if cut < 0 {
		return strings.LastIndexByte(rest, '@')
	}
	if atSign := strings.LastIndexByte(rest[:cut], '@'); atSign >= 0 {
		return atSign
	}
	colon := strings.IndexByte(rest[:cut], ':')
	atSign := strings.IndexByte(rest[cut:], '@')
	if colon < 0 || atSign < 0 || isPort(rest[:colon], rest[colon+1:cut]) {
		return -1
	}
	atSign += cut
	host := rest[atSign+1:]
	if end := strings.IndexAny(host, "/?#"); end >= 0 {
		host = host[:end]
	}
	if last := strings.LastIndexByte(host, '@'); last >= 0 {
		atSign += 1 + last
	}
	return atSign
}

// isPort reports whether host:port reads as a host name and its port: one
// to five digits after a name holding a dot, or localhost. A single-label
// name before digits reads as a user and the start of a password.
func isPort(host, port string) bool {
	if port == "" || len(port) > 5 || strings.Trim(port, "0123456789") != "" {
		return false
	}
	return strings.Contains(host, ".") || strings.EqualFold(host, "localhost")
}

// schemelessToken returns token with a user:password@ written without a
// scheme replaced: at the start of the token, or after an equals sign or
// an opening bracket (--proxy=user:secret@proxy:3128). The last @ before
// the first / ends the password, as a password may hold an @. A name with
// no password (git@github.com:o/r.git, alice@example.com), an empty name
// and a mailto: address are kept.
func schemelessToken(token, placeholder string) string {
	limit := strings.IndexByte(token, '/')
	if limit < 0 {
		limit = len(token)
	}
	atSign := strings.LastIndexByte(token[:limit], '@')
	if atSign < 0 || atSign+1 >= len(token) {
		return token
	}
	colon := strings.IndexByte(token[:atSign], ':')
	if colon < 0 {
		return token
	}
	start := strings.LastIndexAny(token[:colon], "=([{,") + 1
	if colon == start || colon+1 == atSign || strings.EqualFold(token[start:colon], "mailto") {
		return token
	}
	return token[:start] + withAtSign(placeholder) + token[atSign+1:]
}

// withheldParameters returns text, the part of a token from its first
// "://" on, with the value of each query or fragment parameter that
// credentialName names replaced by placeholder. A parameter starts after
// ?, & or # and its value ends at the next & or #; an empty value is kept.
func withheldParameters(text, placeholder string) string {
	var out strings.Builder
	for {
		start := strings.IndexAny(text, "?&#")
		if start < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:start+1])
		text = text[start+1:]
		name, value, found := strings.Cut(text, "=")
		if !found || strings.ContainsAny(name, "?&#/") || !credentialName(name) {
			continue
		}
		end := strings.IndexAny(value, "&#")
		if end < 0 {
			end = len(value)
		}
		out.WriteString(name + "=")
		if end > 0 {
			out.WriteString(placeholder)
		}
		text = value[end:]
	}
}

// credentialName reports whether a query or fragment parameter's name,
// compared without regard to ASCII case, names a credential: token,
// access_token, refresh_token, key, api_key, apikey, api-key, secret,
// client_secret, password, auth or authorization. The list holds what the
// agents' research documents name as credentials in a URL or a request: a
// token in a query string (research/claude-desktop.md, research/openclaw.md),
// an API key in a query string and the access and refresh tokens of a token
// refresh (research/gemini-cli.md), and the API key, secret, token,
// password and authorization names the agents' own exports redact
// (research/pi.md, research/muse-code.md).
func credentialName(name string) bool {
	lowered := []byte(name)
	for i, char := range lowered {
		if 'A' <= char && char <= 'Z' {
			lowered[i] = char + 'a' - 'A'
		}
	}
	switch string(lowered) {
	case "token", "access_token", "refresh_token", "key", "api_key", "apikey", "api-key", "secret",
		"client_secret", "password", "auth", "authorization":
		return true
	}
	return false
}
