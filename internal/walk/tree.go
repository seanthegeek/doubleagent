package walk

import (
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// readError is a directory the walk could not list or enter, and why.
type readError struct {
	path   string
	reason string
}

// treeWalk is what one walk found, in the order find visits paths: depth
// first, each directory before its entries, entries in directory order.
type treeWalk struct {
	start      string
	patterns   []string
	excluded   []string
	readErrors []readError
	files      []string
	unreadable int
}

// CollectTree walks dir for user under home, attributed to agent, as
// collect_tree's find does, and writes its rows. It returns how many
// directories the walk could not read, the count of find's error lines the
// sh collector reports for a Docker volume.
func (collector *Collector) CollectTree(ctx context.Context, user, home, agent, dir string,
) (int, error) {
	owner := scope{user: user, home: home, agent: agent, stager: collector.Stager}
	return collector.collectTree(ctx, owner, dir)
}

// collectTree walks dir in one scope and writes its rows. Under rowsFollowWalk
// each row is written as the walk finds its path. Otherwise the excluded rows,
// the error_read rows and the file rows are written each group whole, in the
// order the platform's find runs its batched -exec programs at the end of a
// walk. The sh collector's find also runs a batch each time its argument list
// fills, so on a tree larger than one batch its rows and log lines interleave
// mid-walk, where this writes each group whole at the end: a documented
// deviation.
func (collector *Collector) collectTree(ctx context.Context, owner scope, dir string,
) (int, error) {
	found := &treeWalk{start: dir, patterns: collector.excludePatterns(owner.home)}
	if err := collector.visit(ctx, owner, found, dir, true); err != nil {
		return found.unreadable, err
	}
	if collector.Inventory {
		collector.recordFiles(owner, found.files)
		return found.unreadable, nil
	}
	if rowsFollowWalk {
		return found.unreadable, nil
	}
	groups := []func() error{
		func() error { return collector.writeExcluded(ctx, owner, found.excluded) },
		func() error { return collector.writeReadErrors(owner, found.readErrors) },
		func() error { return collector.stageFiles(owner, found.files, collector.Secret) },
	}
	if findRunsLastBatchFirst {
		groups[0], groups[2] = groups[2], groups[0]
	}
	for _, group := range groups {
		if err := group(); err != nil {
			return found.unreadable, err
		}
	}
	return found.unreadable, nil
}

// foundKind is what the walk found at a path: an excluded path, a directory
// it cannot read, or a file to stage.
type foundKind int

const (
	foundExcluded foundKind = iota
	foundReadError
	foundFile
)

// record keeps what the walk found at path (reason is a read error's text).
// Under rowsFollowWalk, outside --inventory, the row is written at once, so
// rows and their copy failed: and symlink not recreated: lines come out in
// walk order, as Add-CollectedTree writes them; otherwise the path joins its
// group, written when the walk ends. Under --inventory a read error is logged
// at once where inventoryLogsReadErrors says so, and gets no row.
func (collector *Collector) record(ctx context.Context, owner scope, found *treeWalk,
	kind foundKind, path, reason string,
) error {
	if collector.Inventory && inventoryLogsReadErrors && kind == foundReadError {
		return collector.logReadError(path, reason)
	}
	if rowsFollowWalk && !collector.Inventory {
		switch kind {
		case foundExcluded:
			return collector.writeExcluded(ctx, owner, []string{path})
		case foundReadError:
			return collector.writeReadErrors(owner, []readError{{path, reason}})
		}
		return collector.stageFiles(owner, []string{path}, collector.Secret)
	}
	switch kind {
	case foundExcluded:
		found.excluded = append(found.excluded, path)
	case foundReadError:
		found.readErrors = append(found.readErrors, readError{path, reason})
	default:
		found.files = append(found.files, path)
	}
	return nil
}

// visit applies collect_tree's find expression to path: a claimed path
// below the start is pruned without a row, an excluded one is pruned with
// one, a directory is listed unless it cannot be listed or entered (which
// gives an error_read row instead), and anything else is a file. Under
// inventory only regular files are kept and no row is planned.
func (collector *Collector) visit(ctx context.Context, owner scope, found *treeWalk,
	path string, isDir bool,
) error {
	if _, isClaim := owner.claims[syspath.Fold(path)]; isClaim && path != found.start {
		return nil
	}
	if matchesAny(found.patterns, path) {
		return collector.record(ctx, owner, found, foundExcluded, path, "")
	}
	if !isDir {
		return collector.record(ctx, owner, found, foundFile, path, "")
	}
	if reason, failed := fsmeta.DirReadError(path); failed {
		if err := collector.record(ctx, owner, found, foundReadError, path, reason); err != nil {
			return err
		}
		return collector.findUnreadDir(found, path)
	}
	return collector.visitEntries(ctx, owner, found, path)
}

// findUnreadDir handles a directory the walk cannot read. One that cannot be
// listed gets find's line, `find: <dir>: <reason>`, and adds one to the
// unreadable count CollectTree returns, a Docker volume's figure, which so
// counts one per directory that could not be listed. One that can be listed
// but not entered adds no line and no count, where GNU find writes a line per
// entry, and its entries get no line, row or inventory record: GNU find stats
// some of them, which the sh collector records as error_copy rows, BSD find
// none, and neither is evidence, since no entry can be read. The directory
// keeps its error_read row from visit.
func (collector *Collector) findUnreadDir(found *treeWalk, dir string) error {
	handle, err := osfile.Open(dir)
	if err != nil {
		return collector.findError(found, dir, err.Error())
	}
	return syspath.NativeErrors(handle.Close())
}

// findError counts a path the walk could not read and logs the line find
// writes to the log for it, `find: <path>: <reason>`, with the system's
// reason as Go words it and the log's time added. find writes to the log
// file alone, never to standard error, so a logger that can do the same
// gets the line that way.
func (collector *Collector) findError(found *treeWalk, path, reason string) error {
	found.unreadable++
	if position := strings.LastIndex(reason, ": "); position >= 0 {
		reason = reason[position+2:]
	}
	line := "find: " + syspath.ToNative(path) + ": " + reason
	if log, ok := collector.Log.(fileLogger); ok {
		return log.FileLine(line)
	}
	return collector.Log.Line(line)
}

// visitEntries lists directory and visits each entry in the platform's
// listing order: find's directory order on Unix, Get-ChildItem's
// directories-then-files order on Windows.
func (collector *Collector) visitEntries(ctx context.Context, owner scope, found *treeWalk,
	directory string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	handle, err := osfile.Open(directory)
	if err != nil {
		return collector.findError(found, directory, err.Error())
	}
	entries, err := handle.ReadDir(-1)
	_ = handle.Close()
	if err != nil {
		if err := collector.findError(found, directory, err.Error()); err != nil {
			return err
		}
	}
	for _, entry := range fsmeta.ListingOrder(entries) {
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
