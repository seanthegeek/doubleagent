package gitmeta

import (
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// The paths of the two files under a project directory, in git's own
// spelling; a collection that keeps the on-disk spelling may give them in
// another case.
const (
	ConfigRel = ".git/config"
	HeadRel   = ".git/HEAD"
)

// Remote is one remote of a .git/config, its values in file order.
// PushURLs is what the file says, not git's fallback to URLs. Neither
// slice is nil.
type Remote struct {
	Name     string
	URLs     []string
	PushURLs []string
}

// addValue adds a url or pushurl value to the named remote, creating it;
// any other key is ignored.
func addValue(byName map[string]*Remote, name, key, value string) {
	if key != "url" && key != "pushurl" {
		return
	}
	remote := byName[name]
	if remote == nil {
		remote = &Remote{Name: name, URLs: []string{}, PushURLs: []string{}}
		byName[name] = remote
	}
	if key == "url" {
		remote.URLs = append(remote.URLs, value)
	} else {
		remote.PushURLs = append(remote.PushURLs, value)
	}
}

// sortedRemotes returns the remotes sorted by name.
func sortedRemotes(byName map[string]*Remote) []Remote {
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	remotes := make([]Remote, 0, len(names))
	for _, name := range names {
		remotes = append(remotes, *byName[name])
	}
	return remotes
}

// physicalLines splits text at LF or CRLF only, as git does, dropping a
// leading byte-order mark.
func physicalLines(text string) []string {
	return textlines.Split(strings.TrimPrefix(text, "\ufeff"))
}

// ParseConfig returns the remotes of a .git/config text, sorted by name:
// one per remote section that sets a non-empty url or pushurl. It also
// returns the values git would reject, each described as
// "line <n>: <reason>" and left out of the remotes.
func ParseConfig(text string) ([]Remote, []string) {
	byName := map[string]*Remote{}
	var problems []string
	current, inRemote := "", false
	lines := physicalLines(text)
	for index := 0; index < len(lines); index++ {
		line, ok := strings.TrimLeft(lines[index], gitSpace), true
		for ok && strings.HasPrefix(line, "[") {
			var found header
			found, ok = readHeader(line)
			current, inRemote = found.remote()
			line = strings.TrimLeft(line[found.end:], gitSpace)
		}
		if !ok {
			continue // unterminated or malformed: its keys belong to no section
		}
		key, value, hasValue := readKey(line)
		if !hasValue {
			continue
		}
		start := index + 1
		var reason string
		value, index, reason = readValue(lines, index, value)
		if reason != "" {
			problems = append(problems, "line "+strconv.Itoa(start)+": "+reason)
		} else if inRemote && value != "" {
			addValue(byName, current, key, value)
		}
	}
	return sortedRemotes(byName), problems
}

// ParseHead returns the branch a .git/HEAD text names, another ref in
// full, or the commit hash of a detached HEAD, from its first non-blank
// line; empty text gives "". Only ASCII whitespace is trimmed, since a
// ref may hold other whitespace.
func ParseHead(text string) string {
	first := ""
	for _, line := range physicalLines(text) {
		if line = strings.Trim(line, textlines.ASCIISpace); line != "" {
			first = line
			break
		}
	}
	ref, ok := strings.CutPrefix(first, "ref:")
	if !ok {
		return first
	}
	ref = strings.Trim(ref, textlines.ASCIISpace)
	if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return branch
	}
	return ref
}
