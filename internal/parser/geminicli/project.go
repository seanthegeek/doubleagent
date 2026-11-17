package geminicli

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
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
	base := artifact.Home.PathOf(match["base"])
	return func(names ...string) string {
		return filepath.Join(append([]string{base}, names...)...)
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
	return pytext.DecodeUTF8(data), true
}

// resolveProject returns the project path of tmp/<slug> under the runtime
// directory that base locates paths in: tmp/<slug>/.project_root, else the projects.json key
// whose value is the slug, else the key whose sha256 is the slug or
// projectHash, else projectHash. A side file that exists but cannot be read
// adds its problem for the caller's rows.
func resolveProject(base func(names ...string) string, slug, projectHash string,
	problems *parser.Problems) string {
	if root, ok := readSmall(base("tmp", slug, ".project_root"), problems); ok {
		if root = pytext.Strip(root); root != "" {
			return root
		}
	}
	var projects []pyjson.Member
	if raw, ok := readSmall(base("projects.json"), problems); ok && raw != "" {
		if data, err := pyjson.DecodeString(raw); err == nil {
			projects = data.Get("projects").Members()
		}
	}
	for _, project := range projects {
		if isText(project.Value, slug) {
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

// unquote decodes the %XX escapes of a file name as Python's
// urllib.parse.unquote does: each run of ASCII characters has its valid
// escapes turned into bytes and is decoded as UTF-8 with replacement; an
// escape that is not two hex digits stays as written, and any other
// character is kept.
func unquote(text string) string {
	if !strings.Contains(text, "%") {
		return text
	}
	var out strings.Builder
	for start := 0; start < len(text); {
		end := start
		for end < len(text) && text[end] < 0x80 {
			end++
		}
		if end == start {
			for end < len(text) && text[end] >= 0x80 {
				end++
			}
			out.WriteString(text[start:end])
		} else {
			out.WriteString(pytext.DecodeUTF8(unescapeBytes(text[start:end])))
		}
		start = end
	}
	return out.String()
}

// unescapeBytes returns ASCII text with each %XX escape of two hex digits
// replaced by its byte.
func unescapeBytes(text string) []byte {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '%' && i+2 < len(text) {
			if value, err := hex.DecodeString(text[i+1 : i+3]); err == nil {
				out = append(out, value[0])
				i += 2
				continue
			}
		}
		out = append(out, text[i])
	}
	return out
}
