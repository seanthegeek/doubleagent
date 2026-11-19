//go:build parity

package replay

import (
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// millisecondLayout is a timestamp as the outputs write one.
const millisecondLayout = "2006-01-02T15:04:05.000Z"

// cutTimes holds the timestamps, cut to the millisecond as the binary
// writes them, of a case's sources whose time has digits below the
// millisecond: the modification times of the files of its tree, and the
// times its files' contents write as text with more than three fraction
// digits, as an ISO 8601 time or as epoch seconds. The recording rounded
// such a time to the millisecond and could carry it into the next one;
// the binary cuts it, so only such a timestamp may be one millisecond
// earlier than the recorded one (difference 119). The set is built when
// first asked for, since few comparisons need it; a nil one holds none.
type cutTimes struct {
	build func() (map[string]bool, map[string][]string)
	// times holds every cut time; mtimes gives, for each that is a
	// modification time, the files modified then, by their path under the
	// root with forward slashes.
	times  map[string]bool
	mtimes map[string][]string
}

// holds reports whether timestamp is a time of a source with digits below
// the millisecond, cut.
func (c *cutTimes) holds(timestamp string) bool {
	if c == nil {
		return false
	}
	if c.times == nil {
		c.times, c.mtimes = c.build()
	}
	return c.times[timestamp]
}

// holdsModified reports whether timestamp is the modification time, with
// digits below the millisecond, cut, of the file a row names as its
// source: a row may take its own file's time, not another's. A row names
// its file from the input, which may lie inside the case's tree or hold
// it, so the two paths agree when either ends in the other at a slash.
func (c *cutTimes) holdsModified(timestamp, sourceFile string) bool {
	if !c.holds(timestamp) {
		return false
	}
	source := filepath.ToSlash(sourceFile)
	return slices.ContainsFunc(c.mtimes[timestamp], func(relative string) bool {
		return source == relative || strings.HasSuffix(source, "/"+relative) ||
			strings.HasPrefix(source, "/") && strings.HasSuffix("/"+relative, source)
	})
}

// fixedCutTimes returns a set holding the given cut timestamps, none of
// them a file's modification time.
func fixedCutTimes(timestamps ...string) *cutTimes {
	set := map[string]bool{}
	for _, timestamp := range timestamps {
		set[timestamp] = true
	}
	return &cutTimes{times: set, mtimes: map[string][]string{}}
}

// fixedModifiedTimes returns a set holding the cut timestamp as the
// modification time of each of files, given by their path under the root.
func fixedModifiedTimes(timestamp string, files ...string) *cutTimes {
	set := fixedCutTimes(timestamp)
	set.mtimes[timestamp] = files
	return set
}

// cutTimesOf returns the cutTimes of a recorded tree and its times object;
// a case without a tree has none.
func (c *Corpus) cutTimesOf(tree, times string) *cutTimes {
	if tree == "" {
		return nil
	}
	return &cutTimes{build: func() (map[string]bool, map[string][]string) {
		set, modified := map[string]bool{}, map[string][]string{}
		if times != "" {
			if mtimes, err := c.ReadTimes(times); err == nil {
				for relative, mtime := range mtimes {
					if mtime%int64(time.Millisecond) != 0 {
						cut := time.Unix(0, mtime).UTC().Format(millisecondLayout)
						set[cut] = true
						modified[cut] = append(modified[cut], relative)
					}
				}
			}
		}
		c.addWrittenTimes(tree, set)
		return set, modified
	}}
}

// addWrittenTimes adds to set the times with digits below the millisecond
// that the files of a tree object write as text, cut.
func (c *Corpus) addWrittenTimes(tree string, set map[string]bool) {
	entries, err := c.ReadTree(tree)
	if err != nil {
		return
	}
	for _, entry := range entries {
		switch {
		case entry.Kind == "dir":
			c.addWrittenTimes(entry.Tree, set)
		case entry.Kind == "file" && entry.Blob != "":
			if data, err := c.ReadBlob(entry.Blob, ""); err == nil {
				for _, timestamp := range writtenCutTimes(string(data)) {
					set[timestamp] = true
				}
			}
		}
	}
}

// Times written with more than three fraction digits: an ISO 8601 time,
// UTC when it names no offset, and epoch seconds with a fraction.
var (
	writtenISOTime   = regexp.MustCompile(`([0-9]{4}-[0-9]{2}-[0-9]{2})[T ]([0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{4,9})(Z|[+-][0-9]{2}:?[0-9]{2})?`)
	writtenEpochTime = regexp.MustCompile(`(?:^|[^0-9.])([0-9]{9,10})\.([0-9]{4,9})(?:[^0-9]|$)`)
)

// writtenCutTimes returns the times of writtenISOTime and
// writtenEpochTime in text, cut to the millisecond.
func writtenCutTimes(text string) []string {
	var found []string
	for _, match := range writtenISOTime.FindAllStringSubmatch(text, -1) {
		zone := strings.Replace(match[3], ":", "", 1)
		if zone == "" || zone == "Z" {
			zone = "+0000"
		}
		moment, err := time.Parse("2006-01-02 15:04:05.999999999-0700", match[1]+" "+match[2]+zone)
		if err == nil && moment.Nanosecond()%int(time.Millisecond) != 0 {
			found = append(found, moment.UTC().Format(millisecondLayout))
		}
	}
	for _, match := range writtenEpochTime.FindAllStringSubmatch(text, -1) {
		seconds, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			continue
		}
		fraction := (match[2] + "000000000")[:9]
		nanoseconds, err := strconv.ParseInt(fraction, 10, 64)
		if err != nil || nanoseconds%int64(time.Millisecond) == 0 {
			continue
		}
		found = append(found, time.Unix(seconds, nanoseconds).UTC().Format(millisecondLayout))
	}
	return found
}
