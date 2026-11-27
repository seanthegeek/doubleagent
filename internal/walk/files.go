package walk

import (
	"context"
	"errors"
	"io/fs"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// plannedFile is one file being staged: its lstat and owner before the
// copy, the outcome planned (an empty status means the file is to be
// copied), why its link could not be recreated, and what the copy gave: the
// hash, the bytes written or the error.
type plannedFile struct {
	path      string
	meta      fsmeta.Meta
	owner     string
	kind      string
	status    manifest.Status
	secret    bool
	target    string
	linkError string
	sha256    string
	written   int64
	copyError string
	// member is the archive member the stager put the copy or link in,
	// spelled as the staging tree names it.
	member string
}

// innermostError returns the text of the innermost *fs.PathError in err, the
// text the manifest's error field holds, or err's own text when there is none.
func innermostError(err error) string {
	var pathError *fs.PathError
	for errors.As(err, &pathError) {
		inner := pathError.Err
		var deeper *fs.PathError
		if !errors.As(inner, &deeper) {
			return pathError.Error()
		}
		err = inner
	}
	return err.Error()
}

// planLink reads the target of the link file names and recreates the link at
// its staged path. A link that cannot be read or recreated keeps the reason,
// on every platform, so its row names no archive path and the log says why
// (exception 50).
func planLink(owner scope, file *plannedFile) {
	target, err := osfile.Readlink(file.path)
	file.target = target
	if err == nil {
		file.member, err = owner.stager.Link(file.path, target, file.meta.DirectoryLink())
	}
	if err != nil {
		file.linkError = innermostError(syspath.NativeErrors(err))
	}
}

// specialKind names a special file's kind as the manifest type does.
func specialKind(kind fsmeta.Kind) string {
	switch kind {
	case fsmeta.Socket:
		return "socket"
	case fsmeta.Fifo:
		return "fifo"
	case fsmeta.Device:
		return "device"
	}
	return "other"
}

// lstatOrZero returns path's lstat and true, or empty metadata, every
// number 0 and mode "", and false when path cannot be described.
func lstatOrZero(path string) (fsmeta.Meta, bool) {
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return fsmeta.Meta{}, false
	}
	return meta, true
}

// planFile applies the check order to one path: a symlink (recreated in the
// staging tree), a special file, a secret under --no-secrets, a file over the
// size cap, else a copy. A path lstat cannot read is planned as a copy, which
// then fails with the reason. forceSecret flags the file secret whatever its
// name.
func (collector *Collector) planFile(owner scope, path string, forceSecret bool) plannedFile {
	meta, exists := owner.before[path]
	if !exists {
		meta, exists = lstatOrZero(path)
	}
	file := plannedFile{path: path, meta: meta, kind: "file", owner: collector.ownerOf(path)}
	file.secret = forceSecret || collector.isSecret(owner, path)
	switch {
	case exists && meta.Kind == fsmeta.Symlink:
		file.status, file.kind = manifest.Symlink, "symlink"
		planLink(owner, &file)
	case exists && meta.Kind != fsmeta.File:
		file.status, file.kind = manifest.SkippedSpecial, specialKind(meta.Kind)
	case file.secret && collector.NoSecrets:
		file.status = manifest.SkippedSecret
	case collector.MaxSize > 0 && meta.Size > collector.MaxSize:
		file.status = manifest.SkippedSize
	}
	return file
}

// metaRow is a row of owner for path with the metadata of meta. Attributes
// are written as .NET names them when there are any; Unix has none.
func metaRow(owner scope, path, kind string, meta fsmeta.Meta) manifest.Row {
	row := manifest.Row{
		User: owner.user, Home: owner.home, Agent: owner.agent, Path: path, Type: kind,
		Size: meta.Size, Mtime: meta.Mtime, Atime: meta.Atime, Ctime: meta.Ctime,
		Btime: meta.Btime, UID: meta.UID, GID: meta.GID, Mode: meta.Mode,
	}
	if meta.Attributes != 0 {
		row.Attributes = fsmeta.AttributesString(meta.Attributes)
	}
	return row
}

// fileRow is the manifest row of one staged file, its archive path the
// member the stager put it in, spelled as the archive names it. A link that
// could not be recreated has its reason as the error and no archive path.
func fileRow(owner scope, file plannedFile) manifest.Row {
	row := metaRow(owner, file.path, file.kind, file.meta)
	row.Secret, row.Status, row.Target = file.secret, file.status, file.target
	row.SHA256, row.Error, row.Owner = file.sha256, file.copyError, file.owner
	if file.linkError != "" {
		row.Error = "not recreated in the archive: " + file.linkError
	}
	if file.status == manifest.Collected {
		// The bytes copied, which sha256 covers: a file another process
		// writes may have grown since its lstat.
		row.Size = file.written
	}
	if file.status == manifest.Collected || (file.status == manifest.Symlink && row.Error == "") {
		row.ArchivePath = file.member
	}
	return row
}

// stageFile stages one path and writes its manifest row: it plans the path
// (planFile), logs a link that could not be recreated, copies a file planned
// for a copy, logging a failed copy, and writes the row. forceSecret flags
// the file secret whatever its name. A copy Windows opened outside the image
// root gets no row. Every route that stages a path ends here, so the Holds
// test here is the guarantee that no route stages one of the run's own
// outputs; the tests made before a path gets here only keep the run from
// listing or searching them.
func (collector *Collector) stageFile(owner scope, path string, forceSecret bool) error {
	if collector.Output.Holds(path) {
		return nil
	}
	file := collector.planFile(owner, path, forceSecret)
	if file.linkError != "" {
		line := "symlink not recreated: " + syspath.ToNative(file.path) + ": " + file.linkError
		if err := collector.Log.Line(line); err != nil {
			return err
		}
	}
	if file.status == "" {
		copied, err := owner.stager.Copy(file.path)
		if _, refused := follow.OutOfRoot(err); refused {
			// Logged once by the opener's Admit; nothing was copied.
			return nil
		}
		if err == nil {
			file.status, file.sha256, file.written = manifest.Collected, copied.SHA256, copied.Size
			file.member = copied.ArchivePath
		} else {
			file.status, file.copyError = manifest.ErrorCopy, innermostError(syspath.NativeErrors(err))
			line := "copy failed: " + syspath.ToNative(file.path) + ": " + file.copyError
			if err := collector.Log.Line(line); err != nil {
				return err
			}
		}
	}
	return collector.Manifest.Write(fileRow(owner, file))
}

// excludedMeta returns the type and lstat metadata of an excluded path's row,
// and whether it is sized as a directory: a link, never followed, is a
// symlink row; an entry marked a directory a dir row; anything else a file
// row.
func excludedMeta(path string) (string, fsmeta.Meta, bool) {
	meta, _ := lstatOrZero(path)
	switch {
	case meta.Kind == fsmeta.Symlink:
		return "symlink", meta, false
	case meta.MarkedDirectory():
		return "dir", meta, true
	}
	return "file", meta, false
}

// writeExcluded writes the skipped_excluded row of path, as excludedMeta
// types it. A link is never followed or sized: its row has the link's own
// lstat and size, its stored target (empty when unreadable) and no archive
// path. A real directory keeps its own lstat metadata and is sized with
// fsmeta.DiskUsageBytes. The owner is never read. It stops with ctx's error
// when ctx is cancelled while a directory is being sized. One of the run's
// own outputs gets no row.
func (collector *Collector) writeExcluded(ctx context.Context, owner scope, path string) error {
	if collector.Output.Holds(path) {
		return nil
	}
	kind, meta, isDir := excludedMeta(path)
	if isDir {
		size, err := fsmeta.DiskUsageBytes(ctx, collector.Stager.Opener, path)
		if err != nil {
			return err
		}
		meta.Size = size
	}
	row := metaRow(owner, path, kind, meta)
	row.Status = manifest.SkippedExcluded
	if kind == "symlink" {
		row.Target, _ = osfile.Readlink(path)
	}
	return collector.Manifest.Write(row)
}

// logReadError logs the read failed: line of a directory that cannot be
// listed or entered.
func (collector *Collector) logReadError(path, reason string) error {
	return collector.Log.Line("read failed: " + syspath.ToNative(path) + ": " + reason)
}

// writeReadError writes the error_read row of a directory, with its lstat
// metadata, size 0 and, when its own entry is marked a directory (so a
// junction whose target is gone keeps its owner), its owner, and its read
// failed: line; one of the run's own outputs gets neither.
func (collector *Collector) writeReadError(owner scope, path, reason string) error {
	if collector.Output.Holds(path) {
		return nil
	}
	meta, _ := lstatOrZero(path)
	row := metaRow(owner, path, "dir", meta)
	row.Size, row.Status, row.Error = 0, manifest.ErrorRead, reason
	if meta.MarkedDirectory() {
		row.Owner = collector.ownerOf(path)
	}
	if err := collector.Manifest.Write(row); err != nil {
		return err
	}
	return collector.logReadError(path, reason)
}

// recordFile gives a regular file's size and modification time to the
// inventory records. A path lstat describes as anything else (a reparse
// point Windows lists as a regular file) is not counted.
func (collector *Collector) recordFile(owner scope, path string) {
	meta, exists := lstatOrZero(path)
	if exists && meta.Kind != fsmeta.File {
		return
	}
	collector.Records.File(owner.agent, max(meta.Size, 0), max(meta.Mtime, 0))
}

// DirRow is the row of the directory path for user and home with no agent,
// built from its lstat meta as an excluded directory's row is.
func DirRow(user, home, path string, meta fsmeta.Meta) manifest.Row {
	return metaRow(scope{user: user, home: home}, path, "dir", meta)
}
