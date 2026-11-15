package hostcmd

import "strings"

// Plutil reads launchd property lists through plutil, the property list tool
// macOS ships, so a value is the one launchd reads (binary plists, comments,
// CDATA sections and a repeated key's last value included). No plist is ever
// parsed by hand.
type Plutil struct {
	runner Runner
}

// NewPlutil returns a Plutil that runs plutil through runner.
func NewPlutil(runner Runner) Plutil {
	return Plutil{runner: runner}
}

// Available reports whether plutil is on the search path; it is not off macOS.
func (plutil Plutil) Available() bool {
	return plutil.runner.Available("plutil")
}

// Lint reports whether `plutil -lint` accepts the file at path.
func (plutil Plutil) Lint(path string) bool {
	_, err := plutil.runner.Output("plutil", "-lint", path)
	return err == nil
}

// ExtractString reads the value at keyPath (`EnvironmentVariables.NAME`,
// `UserName`) of the plist at path with `plutil -extract KEYPATH xml1`. found
// is false when plutil finds no value there. Otherwise value holds the text of
// a string, or reason says why there is none: `holds a newline` for a string
// spanning lines, `holds an XML entity` for one plutil had to encode, `not a
// string` for any other type.
func (plutil Plutil) ExtractString(path, keyPath string) (value, reason string, found bool) {
	output, err := plutil.runner.Output("plutil", "-extract", keyPath, "xml1", "-o", "-", path)
	if err != nil {
		return "", "", false
	}
	value, reason = extractedString(string(output))
	return value, reason, true
}

// extractedString reads the one-element document plutil prints: the three
// header lines and the closing line are dropped with the sh collector's
// parameter expansions, after the trailing newlines a command substitution
// removes, so a document of another shape reads as `not a string`.
func extractedString(output string) (value, reason string) {
	body := strings.TrimRight(output, "\n")
	for range 3 {
		if _, after, found := strings.Cut(body, "\n"); found {
			body = after
		}
	}
	if last := strings.LastIndexByte(body, '\n'); last >= 0 {
		body = body[:last]
	}
	const open, closing = "<string>", "</string>"
	switch {
	case strings.HasPrefix(body, open) && strings.Contains(body, "\n"):
		return "", "holds a newline"
	case strings.HasPrefix(body, open) && strings.HasSuffix(body, closing) &&
		len(body) >= len(open)+len(closing):
		text := body[len(open) : len(body)-len(closing)]
		if strings.Contains(text, "&") {
			return "", "holds an XML entity"
		}
		return text, ""
	}
	return "", "not a string"
}
