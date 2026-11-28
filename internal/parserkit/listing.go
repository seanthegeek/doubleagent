package parserkit

import (
	"errors"
	"io/fs"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/reason"
)

// ChildListings is the directories below one parent, each listed once,
// with the places of those whose listing holds each name as a regular
// file, so a parser that looks for a name in every one of them costs the
// directories holding it rather than all of them. A directory that could
// not be read holds no name, and its failure is kept for the parser to
// report once. A name is found by the bytes the
// listing holds (through the fold given, if any), so a parser uses it where
// the agent's own writing spells the name as the file does.
type ChildListings struct {
	directories []Directory
	holders     map[string][]int
	failures    []ListingFailure
	fold        func(string) string
}

// ListingFailure is a directory of a ChildListings that could not be read:
// its place, the directory whose reading failed (the parent when the
// directory's own entry could not be looked at) and the error.
type ListingFailure struct {
	Place int
	Path  string
	Err   error
}

// Problem returns the failure's unreadable problem.
func (f ListingFailure) Problem() Problem {
	return Problem{Message: reason.Unreadable(f.Path, f.Err)}
}

// fail files a directory that could not be read at the next place.
func (c *ChildListings) fail(path string, err error) {
	c.failures = append(c.failures, ListingFailure{Place: len(c.directories), Path: path, Err: err})
	c.directories = append(c.directories, Directory{})
}

// key returns name through the fold, if any.
func (c *ChildListings) key(name string) string {
	if c.fold == nil {
		return name
	}
	return c.fold(name)
}

// add lists one plain directory at the next place, filing it under the
// name of each of its regular files and keeping the listing in the home's
// store for its Child, or as failed when it cannot be listed.
func (c *ChildListings) add(directory Directory) {
	entries, err := ReadDir(directory.Path())
	if err != nil {
		c.fail(directory.Path(), err)
		return
	}
	directory.home.KeepListing(directory.Path(), entries)
	place := len(c.directories)
	c.directories = append(c.directories, directory)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		key := c.key(entry.Name())
		if places := c.holders[key]; len(places) == 0 || places[len(places)-1] != place {
			c.holders[key] = append(places, place)
		}
	}
}

// ListChildren lists, once each and in the order given, the entries of
// parent called names that are plain directories; a name that is not one
// plain component, is absent, or is a link or anything but a directory
// holds nothing. fold, when not nil, is applied to every name filed and
// looked up. Each listing made adds one unit of the named work.
func ListChildren(parent Directory, names []string, fold func(string) string, work Work,
	listings string) *ChildListings {
	children := &ChildListings{holders: map[string][]int{}, fold: fold}
	for _, name := range names {
		path, ok := parent.Child(name)
		if !ok {
			continue
		}
		info, err := osfile.Lstat(path)
		switch {
		case err != nil && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)):
			continue
		case err != nil:
			children.fail(parent.Path(), err)
			continue
		case !info.IsDir() || info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0:
			continue
		}
		work.Add(listings, 1)
		children.add(Directory{path: path, home: parent.home})
	}
	return children
}

// Holders returns the places, in order, of the directories whose listing
// holds name as a regular file.
func (c *ChildListings) Holders(name string) []int {
	return c.holders[c.key(name)]
}

// Directory returns the directory at a place Holders gave.
func (c *ChildListings) Directory(place int) Directory {
	return c.directories[place]
}

// Failures returns the directories that could not be read, in place order.
func (c *ChildListings) Failures() []ListingFailure {
	return c.failures
}
