// Package secrets decides how the analysis output shows a secret found in
// the evidence: a token, key, password or cookie value, the credentials a
// URL carries, and a reader's error for a file that holds secrets, whose
// message can quote the file. By default each is shown as the evidence
// holds it, since it helps establish what an agent reached. When the
// analyst asks to withhold secrets (timeline --no-secrets, detect
// --no-secrets), each is replaced by one placeholder, REDACTED unless the
// analyst names another, so every withheld secret looks alike and can be
// searched for; the row or line that held it stays.
//
// The methods of Setting are the one place that decides. A parser reaches
// them through the parser kit (parser.SecretText, parser.URLText and
// parser.ReaderErrorText), which reads the setting from the artifact's home;
// a parser never tests the setting or writes the placeholder itself.
//
// A URL is cut by hand rather than read with net/url, which misreads or
// refuses a URL whose password holds a raw slash and refuses one with a bad
// percent-escape, and finds no URL inside longer text such as a command
// line; each time it would leave the credentials shown.
//
// Withholding covers only what these rules identify. A credential written
// in free text that no rule names (a prompt, a command line, a tool's
// output, a pasted file) is shown either way, so a person reviews a
// timeline before it is shared, whichever mode wrote it.
package secrets

import (
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/tomldoc"
)

// DefaultPlaceholder is the text a withheld secret is replaced by unless
// the analyst names another.
const DefaultPlaceholder = "REDACTED"

// Setting is how one analysis run shows secrets. Its zero value shows
// them; with Withhold set, each secret a rule identifies is replaced by
// Placeholder, which may be empty.
type Setting struct {
	Withhold    bool
	Placeholder string
}

// Value returns a secret value read from the evidence (a token, key,
// password or cookie) as the output shows it: as written, or the
// placeholder when the setting withholds secrets. An empty value is no
// secret and stays empty.
func (s Setting) Value(value string) string {
	if !s.Withhold || value == "" {
		return value
	}
	return s.Placeholder
}

// URLs returns text, a URL, a host string or longer text holding URLs, as
// the output shows it: as written, or with the credentials of each URL in
// it replaced by the placeholder when the setting withholds secrets
// (withheldURLs says what counts as credentials).
func (s Setting) URLs(text string) string {
	if !s.Withhold {
		return text
	}
	return withheldURLs(text, s.Placeholder)
}

// ReaderError returns the message of err, the error a JSON, TOML or XML
// reader gave for a file that may hold secrets, as the output shows it: in
// full, or, when the setting withholds secrets, where the reader stopped
// ("invalid JSON at line 3, column 9") followed by the placeholder in place
// of the reader's detail, which can quote the file. A TOML document refused
// for a size or depth limit (tomldoc.Error.IsLimit) quotes nothing from the
// file and is always given in full; an error of another kind is the
// placeholder alone after "error".
func (s Setting) ReaderError(err error) string {
	var tomlError *tomldoc.Error
	isTOML := errors.As(err, &tomlError)
	if !s.Withhold || isTOML && tomlError.IsLimit() {
		return err.Error()
	}
	var jsonError *jsonvalue.Error
	var xmlError *xml.SyntaxError
	switch {
	case errors.As(err, &jsonError):
		return s.after(fmt.Sprintf("invalid JSON at line %d, column %d", jsonError.Line, jsonError.Column))
	case isTOML && tomlError.Line > 0:
		return s.after(fmt.Sprintf("invalid TOML at line %d, column %d", tomlError.Line, tomlError.Column))
	case isTOML:
		return s.after("invalid TOML")
	case errors.As(err, &xmlError):
		return s.after(fmt.Sprintf("invalid XML at line %d", xmlError.Line))
	}
	return s.after("error")
}

// after returns where a reader stopped followed by ": " and the
// placeholder, or alone when the placeholder is empty.
func (s Setting) after(position string) string {
	if s.Placeholder == "" {
		return position
	}
	return position + ": " + s.Placeholder
}
