//go:build parity

package replay

import (
	"regexp"
	"strconv"
	"time"
)

// millisecondLayout is a timestamp as the outputs write one.
const millisecondLayout = "2006-01-02T15:04:05.000Z"

// cutTimes holds the timestamps, cut to the millisecond as the binary
// writes them, of a case's sources whose time the recording could carry
// into the next millisecond (carried): the modification times of the files
// and directories of its tree, and the times its files' contents write as
// text in epoch seconds. The binary cuts such a
// time, so only such a timestamp may be one millisecond earlier than the
// recorded one (difference 119). The set is built when first asked for,
// since few comparisons need it; a nil one holds none.
type cutTimes struct {
	build func() (map[string]bool, map[string][]string)
	// times holds every cut time; mtimes gives, for each that is a
	// modification time, the files and directories modified then, by their
	// path under the root with forward slashes.
	times  map[string]bool
	mtimes map[string][]string
}

// holds reports whether timestamp is a time of a source that is carried,
// cut.
func (c *cutTimes) holds(timestamp string) bool {
	if c == nil {
		return false
	}
	if c.times == nil {
		c.times, c.mtimes = c.build()
	}
	return c.times[timestamp]
}

// holdsModified reports whether timestamp is the modification time, cut,
// of a file or directory of the case's tree that is carried. The file
// is not matched with the one a row names: a row names its file from the
// input, with or without a leading slash, and the input may lie anywhere in
// the tree, so no rule on the two paths holds for every input.
func (c *cutTimes) holdsModified(timestamp string) bool {
	return c.holds(timestamp) && len(c.mtimes[timestamp]) > 0
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

// roundsUp is the least part below the millisecond, in nanoseconds, of a
// time the recording could carry into the next millisecond. The recording
// rounded a time to the microsecond, the only step that rounds up, and
// then cut it to the millisecond, so only a part of 999.5 microseconds or
// more is carried; the bound leaves room for the error of the floating
// point seconds it read the time as, under a quarter of a microsecond.
const roundsUp = 999_000

// carried reports whether a time whose part below the millisecond is part
// nanoseconds is one the recording could carry into the next millisecond.
func carried(part int64) bool { return part >= roundsUp }

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
					if carried(mtime % int64(time.Millisecond)) {
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

// addWrittenTimes adds to set the carried times that the files of a tree
// object write as text, cut.
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

// writtenEpochTime is a time written as epoch seconds with more than three
// fraction digits. A time written as ISO 8601 text is not looked for: the
// recording cut such a fraction and never carried it.
var writtenEpochTime = regexp.MustCompile(`(?:^|[^0-9.])([0-9]{9,10})\.([0-9]{4,9})(?:[^0-9]|$)`)

// writtenCutTimes returns the carried times of writtenEpochTime in text,
// cut to the millisecond.
func writtenCutTimes(text string) []string {
	var found []string
	for _, match := range writtenEpochTime.FindAllStringSubmatch(text, -1) {
		seconds, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			continue
		}
		fraction := (match[2] + "000000000")[:9]
		nanoseconds, err := strconv.ParseInt(fraction, 10, 64)
		if err != nil || !carried(nanoseconds%int64(time.Millisecond)) {
			continue
		}
		found = append(found, time.Unix(seconds, nanoseconds).UTC().Format(millisecondLayout))
	}
	return found
}
