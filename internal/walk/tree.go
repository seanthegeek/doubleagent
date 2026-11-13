package walk

import (
	"context"
	"os"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
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

// collectTree walks dir in one scope, then writes the excluded rows, the
// error_read rows and the file rows, each group whole, in the order the
// platform's find runs its batched -exec programs at the end of a walk.
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
	groups := []func() error{
		func() error { return collector.writeExcluded(owner, found.excluded) },
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

// visit applies collect_tree's find expression to path: a claimed path
// below the start is pruned without a row, an excluded one is pruned with
// one, a directory is listed unless it cannot be listed or entered (which
// gives an error_read row instead), and anything else is a file. Under
// inventory only regular files are kept and no row is planned.
func (collector *Collector) visit(ctx context.Context, owner scope, found *treeWalk,
	path string, isDir bool,
) error {
	if _, isClaim := owner.claims[path]; isClaim && path != found.start {
		return nil
	}
	if matchesAny(found.patterns, path) {
		found.excluded = append(found.excluded, path)
		return nil
	}
	if !isDir {
		found.files = append(found.files, path)
		return nil
	}
	if reason, failed := fsmeta.DirReadError(path); failed {
		found.readErrors = append(found.readErrors, readError{path, reason})
		return collector.findUnreadDir(found, path)
	}
	return collector.visitEntries(ctx, owner, found, path)
}

// findUnreadDir logs the lines find writes for a directory the walk cannot
// read: one for the directory when it cannot be listed; when it can be
// listed but not entered, none for the directory itself and, where find
// reports them, one per entry, whose type find cannot read. An empty such
// directory gives no line on any platform.
func (collector *Collector) findUnreadDir(found *treeWalk, dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return collector.findError(found, dir, err.Error())
	}
	entries, err := handle.ReadDir(-1)
	_ = handle.Close()
	if err != nil {
		if err := collector.findError(found, dir, err.Error()); err != nil {
			return err
		}
	}
	if !findReportsUnenterableEntries {
		return nil
	}
	for _, entry := range entries {
		err := collector.findError(found, dir+"/"+entry.Name(), syscall.EACCES.Error())
		if err != nil {
			return err
		}
	}
	return nil
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
	line := "find: " + path + ": " + reason
	if log, ok := collector.Log.(fileLogger); ok {
		return log.FileLine(line)
	}
	return collector.Log.Line(line)
}

// visitEntries lists directory in directory order and visits each entry.
func (collector *Collector) visitEntries(ctx context.Context, owner scope, found *treeWalk,
	directory string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	handle, err := os.Open(directory)
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
	for _, entry := range entries {
		path := directory + "/" + entry.Name()
		if collector.Inventory && !entry.IsDir() && !entry.Type().IsRegular() {
			continue
		}
		if err := collector.visit(ctx, owner, found, path, entry.IsDir()); err != nil {
			return err
		}
	}
	return nil
}
