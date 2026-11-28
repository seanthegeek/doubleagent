package imagetest

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// fromWork returns target relative to directory, with "/" between its names,
// and true when it is an absolute path under directory; else target as it is.
func fromWork(directory, target string) (string, bool) {
	prefix := directory + string(filepath.Separator)
	if rest, found := strings.CutPrefix(target, prefix); found {
		return filepath.ToSlash(rest), true
	}
	return target, false
}

// readEntry returns the entry at path below directory.
func readEntry(directory, path string) (Entry, error) {
	native := filepath.Join(directory, filepath.FromSlash(path))
	info, err := osfile.Lstat(native)
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{Path: path, ModTime: info.ModTime().UTC()}
	if entry.Kind, err = kindOf(native, info); err != nil {
		return Entry{}, err
	}
	entry.Mode = modeOf(entry.Kind, info)
	switch {
	case isLink(entry.Kind):
		target, err := linkTarget(native, entry.Kind)
		if err != nil {
			return Entry{}, err
		}
		entry.Text, entry.FromWork = fromWork(directory, target)
	case entry.Kind == KindFile:
		content, err := osfile.ReadFile(native)
		if err != nil {
			return Entry{}, err
		}
		entry.Text = string(content)
	}
	return entry, nil
}

// readTree appends the entries below the directory at relative (under
// directory) to entries, in directory order.
func readTree(directory, relative string, entries *[]Entry) error {
	listing, err := osfile.ReadDir(filepath.Join(directory, filepath.FromSlash(relative)))
	if err != nil {
		return err
	}
	for _, item := range listing {
		path := item.Name()
		if relative != "" {
			path = relative + "/" + path
		}
		entry, err := readEntry(directory, path)
		if err != nil {
			return err
		}
		*entries = append(*entries, entry)
		if entry.Kind == KindDir {
			if err := readTree(directory, path, entries); err != nil {
				return err
			}
		}
	}
	return nil
}

// Read returns the entries of the tree under directory, sorted by path, as
// the lines of a manifest would give them: what Build made, or any other
// tree to compare with it. A link whose target lies under directory is read
// as a target from the work directory. It never follows a link.
func Read(directory string) ([]Entry, error) {
	var entries []Entry
	if err := readTree(directory, "", &entries); err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(first, second Entry) int { return strings.Compare(first.Path, second.Path) })
	return entries, nil
}

// kindOfMode returns the kind of a plain (not Windows-specific) entry from its
// mode type bits.
func kindOfMode(path string, mode fs.FileMode) (Kind, error) {
	switch mode.Type() {
	case 0:
		return KindFile, nil
	case fs.ModeDir:
		return KindDir, nil
	case fs.ModeSymlink:
		return KindLink, nil
	case fs.ModeNamedPipe:
		return KindFifo, nil
	case fs.ModeSocket:
		return KindSocket, nil
	}
	return "", fmt.Errorf("%s: an entry of type %v has no manifest kind", path, mode.Type())
}
