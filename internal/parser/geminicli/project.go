package geminicli

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// hex64Pattern is a slug that is the sha256 of the project path, the hash
// directory name older builds use.
var hex64Pattern = regexp.MustCompile(`^[0-9a-f]{64}\z`)

// nameTimePattern is an ISO time spelled in a file name, ":" written "-" and
// "." written "-" or "_".
var nameTimePattern = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})T([0-9]{2})-([0-9]{2})-` +
	`([0-9]{2})[-_]([0-9]{3})Z\z`)

// inBase returns where the collection holds a path below the runtime
// directory that a match of the artifact's path names: a path of the home,
// found through Home.PathOf, which follows the collector's staged paths.
// It gives "" for an artifact with no home.
func inBase(artifact *input.Artifact, match pathMatch) func(names ...string) string {
	if artifact.Home == nil {
		return func(...string) string { return "" }
	}
	base, found := artifact.Home.PathOf(match["base"])
	return func(names ...string) string {
		if path, ok := parser.Under(base, names...); found && ok {
			return path
		}
		return ""
	}
}

// readSmall returns a side file's text decoded as UTF-8 with replacement,
// a byte-order mark kept, or false when it is absent, a link or not a
// regular file. Only a file that cannot be read adds its problem, since
// both side files are optional.
func readSmall(path string, problems *parser.Problems) (string, bool) {
	if !parser.RegularFile(path) {
		return "", false
	}
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return "", false
	}
	return textlines.ValidUTF8(data), true
}

// resolveProject returns the project path of tmp/<slug> under the runtime
// directory that base locates paths in: tmp/<slug>/.project_root, else the projects.json key
// whose value is the slug, else the key whose sha256 is the slug or
// projectHash, else projectHash. A side file that exists but cannot be read
// adds its problem for the caller's rows.
func resolveProject(base func(names ...string) string, slug, projectHash string,
	problems *parser.Problems) string {
	if root, ok := readSmall(base("tmp", slug, ".project_root"), problems); ok {
		if root = strings.TrimSpace(root); root != "" {
			return root
		}
	}
	var projects []jsonvalue.Member
	if raw, ok := readSmall(base("projects.json"), problems); ok && raw != "" {
		if data, err := jsonvalue.DecodeString(raw); err == nil {
			projects = data.Get("projects").Members()
		}
	}
	for _, project := range projects {
		if parser.IsText(project.Value, slug) {
			return project.Key
		}
	}
	wanted := map[string]bool{}
	if hex64Pattern.MatchString(slug) {
		wanted[slug] = true
	}
	if projectHash != "" {
		wanted[projectHash] = true
	}
	for _, project := range projects {
		if len(wanted) == 0 {
			break
		}
		digest := sha256.Sum256([]byte(project.Key))
		if wanted[hex.EncodeToString(digest[:])] {
			return project.Key
		}
	}
	return projectHash
}

// projectOf returns the project of a file under tmp/<slug>/, or "" for one
// outside it.
func projectOf(artifact *input.Artifact, match pathMatch, problems *parser.Problems) string {
	slug, ok := match["slug"]
	if !ok {
		return ""
	}
	return resolveProject(inBase(artifact, match), slug, "", problems)
}

// nameTime returns the timeline time of an ISO time a file name spells with
// "-" or "_" in place of ":" and ".", or "".
func nameTime(text string) string {
	found := nameTimePattern.FindStringSubmatch(text)
	if found == nil {
		return ""
	}
	return timeutil.FromText(found[1] + "T" + found[2] + ":" + found[3] + ":" + found[4] + "." +
		found[5] + "Z")
}

// urlWithoutQuery returns a request URL up to its query string or
// fragment, which can carry an API key.
func urlWithoutQuery(url string) string {
	url, _, _ = strings.Cut(url, "?")
	url, _, _ = strings.Cut(url, "#")
	return url
}

// unquote decodes the escapes of a checkpoint's tag, which the agent
// writes with encodeURIComponent (research/gemini-cli.md), through
// url.PathUnescape; a byte that is not UTF-8 becomes U+FFFD. A name with an
// invalid escape, such as an older checkpoint's "100%", is shown as written.
func unquote(text string) string {
	decoded, err := url.PathUnescape(text)
	if err != nil {
		return text
	}
	return textlines.ValidUTF8([]byte(decoded))
}
