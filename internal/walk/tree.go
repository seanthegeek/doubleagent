package walk

import (
	"context"
	"io/fs"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// treeWalk is one walk's start and how many directories it could not list.
type treeWalk struct {
	start      string
	unreadable int
}

// foundFile stages the file at path, or under --inventory gives it to the
// records.
func (collector *Collector) foundFile(owner scope, path string) error {
	if collector.Inventory {
		collector.recordFile(owner, path)
		return nil
	}
	return collector.stageFile(owner, path, false)
}

// foundReadError writes the error_read row and read failed: line of a
// directory that cannot be listed or entered, or under --inventory the line
// alone.
func (collector *Collector) foundReadError(owner scope, path, reason string) error {
	if collector.Inventory {
		return collector.logReadError(path, reason)
	}
	return collector.writeReadError(owner, path, reason)
}

// countUnlisted adds one to the unreadable count when dir, which already has
// its error_read row, cannot be opened for listing; a directory that can be
// listed but not entered is not counted. Its entries get no row, line or
// inventory record (exception 12), since no entry can be read.
func (collector *Collector) countUnlisted(found *treeWalk, dir string) error {
	handle, err := fsmeta.OpenDir(collector.Stager.Opener, dir)
	if _, refused := follow.OutOfRoot(err); refused {
		return nil
	}
	if err != nil {
		found.unreadable++
		return nil
	}
	return syspath.NativeErrors(handle.Close())
}

// visit decides what the walk does with path: one of the run's own outputs,
// or a claimed path below the start, is pruned without a row, an excluded one
// is pruned with one, a directory is listed unless it cannot be listed or
// entered (which gives an error_read row instead), and anything else is a
// file.
func (collector *Collector) visit(ctx context.Context, owner scope, found *treeWalk,
	path string, isDir bool,
) error {
	if collector.Output.Holds(path) || path != found.start && owner.claims.Has(path) {
		return nil
	}
	if collector.excluded(owner, path) {
		if collector.Inventory {
			return nil
		}
		return collector.writeExcluded(ctx, owner, path)
	}
	if !isDir {
		return collector.foundFile(owner, path)
	}
	// The listing typed path a directory and no reparse point, under a
	// directory already checked, so a refusal can only come from an entry
	// swapped for a link out of the root after the listing: it keeps an
	// error_read row, typed dir with the link's own lstat metadata, and is
	// not opened, so nothing behind the link is read.
	if reason, failed, err := follow.DirReadError(collector.check(), collector.Stager.Opener,
		path); failed {
		if follow.IsOpenedOutside(err) {
			// Logged once by the opener's Admit: no row, nothing listed.
			return nil
		}
		if err := collector.foundReadError(owner, path, reason); err != nil {
			return err
		}
		if _, refused := follow.OutOfRoot(err); refused {
			return nil
		}
		return collector.countUnlisted(found, path)
	}
	return collector.visitEntries(ctx, owner, found, path)
}

// collectTree walks dir in one scope, depth first, each directory before its
// entries and the entries in the order the directory lists them. Each row is
// written as the walk reaches its path, so a copy failed: line follows its
// file's match line.
func (collector *Collector) collectTree(ctx context.Context, owner scope, dir string,
) (int, error) {
	found := &treeWalk{start: dir}
	err := collector.visit(ctx, owner, found, dir, true)
	return found.unreadable, err
}

// CollectTree walks dir for user under home, attributed to agent, and writes
// its rows. It returns how many directories the walk could not list, the
// count reported for a Docker volume.
func (collector *Collector) CollectTree(ctx context.Context, user, home, agent, dir string,
) (int, error) {
	owner := scope{user: user, home: home, agent: agent, stager: collector.Stager}
	return collector.collectTree(ctx, owner, dir)
}

// unlisted counts directory as unreadable and writes its error_read row and
// read failed: line, for a directory whose open or listing failed after
// DirReadError found it readable. The reason is the first line of err's
// text, as DirReadError gives it, since a path can hold a newline.
func (collector *Collector) unlisted(owner scope, found *treeWalk, directory string, err error,
) error {
	found.unreadable++
	reason, _, _ := strings.Cut(syspath.NativeErrors(err).Error(), "\n")
	return collector.foundReadError(owner, directory, reason)
}

// readEntries opens directory and returns its entries in listing order, with
// those read before an error and the error; an open that fails gives none.
// A test's listDirectory takes its place.
func (collector *Collector) readEntries(directory string) ([]fs.DirEntry, error) {
	if collector.listDirectory != nil {
		return collector.listDirectory(directory)
	}
	handle, err := fsmeta.OpenDir(collector.Stager.Opener, directory)
	if err != nil {
		return nil, err
	}
	entries, err := handle.ReadDir(-1)
	_ = handle.Close()
	return entries, err
}

// visitEntries lists directory and visits each entry in the order the
// listing returns them. A listing that fails part way gives the directory
// its error_read row, and the entries read before the failure are visited.
func (collector *Collector) visitEntries(ctx context.Context, owner scope, found *treeWalk,
	directory string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := collector.readEntries(directory)
	if _, refused := follow.OutOfRoot(err); refused {
		// Windows opened it outside the image root: logged once by the
		// opener's Admit, and nothing in it is listed.
		return nil
	}
	if err != nil {
		if err := collector.unlisted(owner, found, directory, err); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		path := directory + "/" + entry.Name()
		isDir := descends(entry)
		if collector.Inventory && !isDir && !entry.Type().IsRegular() {
			continue
		}
		if err := collector.visit(ctx, owner, found, path, isDir); err != nil {
			return err
		}
	}
	return nil
}
