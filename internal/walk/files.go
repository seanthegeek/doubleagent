package walk

import (
	"context"
	"errors"
	"io/fs"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// plannedFile is one file through the staging phases: its lstat and owner
// before the copy, the outcome planned (an empty status means the file is to
// be copied), why its link could not be recreated, and what the copy gave:
// the hash, the bytes written or the error.
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
}

// stageFiles stages paths as STAGE_FILES_WORKER does, in three phases kept
// apart so the copies could run in parallel without changing what is written:
// plan (lstat, staging directories, symlinks, the check order), transfer (the
// copies and their hashes, with a log line per failed copy) and emit (one
// manifest row per path, in the order given).
func (collector *Collector) stageFiles(owner scope, paths []string,
	secret func(string) bool,
) error {
	files := collector.planFiles(owner, paths, secret)
	if err := collector.transferFiles(owner, files); err != nil {
		return err
	}
	for _, file := range files {
		if err := collector.Manifest.Write(fileRow(owner, file)); err != nil {
			return err
		}
	}
	return nil
}

// planFiles makes the staging directory of every path first, then plans each.
func (collector *Collector) planFiles(owner scope, paths []string,
	secret func(string) bool,
) []plannedFile {
	for _, path := range paths {
		_ = owner.stager.MakeParent(path)
	}
	files := make([]plannedFile, len(paths))
	for i, path := range paths {
		files[i] = collector.planFile(owner, path, secret)
	}
	return files
}

// planFile applies the check order to one path: a symlink (recreated in the
// staging tree), a special file, a secret under --no-secrets, a file over the
// size cap, else a copy. A path lstat cannot read is planned as a copy, which
// then fails with the reason.
func (collector *Collector) planFile(owner scope, path string,
	secret func(string) bool,
) plannedFile {
	meta, exists := lstatOrZero(path)
	file := plannedFile{path: path, meta: meta, kind: "file", owner: collector.ownerOf(path)}
	relative, _ := syspath.CutPrefix(path, owner.home+"/")
	file.secret = secret != nil && secret(relative)
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

// planLink reads the target of the link file names and recreates the link at
// its staged path. Where recordsLinkErrors holds, a link that cannot be read
// or recreated keeps the reason, as Add-StagedLink returns it; the sh
// collector ignores both failures and still tries ln -s with an empty target.
func planLink(owner scope, file *plannedFile) {
	target, err := osfile.Readlink(file.path)
	file.target = target
	if err == nil || !recordsLinkErrors {
		err = owner.stager.Symlink(file.path, target, file.meta.DirectoryLink())
	}
	if err != nil && recordsLinkErrors {
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

// transferFiles copies every file planned for a copy and records the hash and
// size or the error, and logs each link that could not be recreated.
func (collector *Collector) transferFiles(owner scope, files []plannedFile) error {
	for i := range files {
		file := &files[i]
		if file.linkError != "" {
			line := "symlink not recreated: " + syspath.ToNative(file.path) + ": " + file.linkError
			if err := collector.Log.Line(line); err != nil {
				return err
			}
		}
		if file.status != "" {
			continue
		}
		hash, written, err := owner.stager.CopyFile(file.path)
		if err == nil {
			file.status, file.sha256, file.written = manifest.Collected, hash, written
			continue
		}
		file.status, file.copyError = manifest.ErrorCopy, innermostError(syspath.NativeErrors(err))
		if err := collector.logCopyFailure(owner, file); err != nil {
			return err
		}
	}
	return nil
}

// logCopyFailure logs the copy failed: line of file. A failed copy's partial
// file is removed when it lies inside the staging directory, and the line
// says when it was not.
func (collector *Collector) logCopyFailure(owner scope, file *plannedFile) error {
	note := ""
	if _, dest := owner.stager.Dest(file.path); !owner.stager.RemoveFailedCopy(dest) {
		note = "; staged path resolves outside the staging directory, not removed"
	}
	return collector.Log.Line("copy failed: " + syspath.ToNative(file.path) + ": " +
		file.copyError + note)
}

// fileRow is the manifest row of one staged file. A link that could not be
// recreated has its reason as the error and no archive path.
func fileRow(owner scope, file plannedFile) manifest.Row {
	row := metaRow(owner, file.path, file.kind, file.meta)
	row.Secret, row.Status, row.Target = file.secret, file.status, file.target
	row.SHA256, row.Error, row.Owner = file.sha256, file.copyError, file.owner
	if file.linkError != "" {
		row.Error = "not recreated in the archive: " + file.linkError
	}
	if file.status == manifest.Collected {
		row.Size = collectedSize(file)
	}
	if file.status == manifest.Collected || (file.status == manifest.Symlink && row.Error == "") {
		row.ArchivePath, _ = owner.stager.Dest(file.path)
	}
	return row
}

// writeExcluded writes one skipped_excluded row per path, as
// EXCLUDED_PATHS_WORKER and Add-ExcludedPath do: a directory as a dir row
// sized with fsmeta.DiskUsageBytes, its other metadata as excludedMeta gives
// it, anything else as a file row with its lstat. The owner is never read.
// It stops with ctx's error when ctx is cancelled while a directory is being
// sized.
func (collector *Collector) writeExcluded(ctx context.Context, owner scope, paths []string,
) error {
	for _, path := range paths {
		kind, meta, isDir := excludedMeta(path)
		if isDir {
			size, err := fsmeta.DiskUsageBytes(ctx, path)
			if err != nil {
				return err
			}
			meta.Size = size
		}
		row := metaRow(owner, path, kind, meta)
		row.Status = manifest.SkippedExcluded
		if err := collector.Manifest.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// writeReadErrors writes one error_read row per directory, with its lstat
// metadata, size 0 and, when it is a directory that can be seen, its owner,
// and its log line, as write_read_error_row and Write-ReadErrorRow do.
func (collector *Collector) writeReadErrors(owner scope, failures []readError) error {
	for _, failure := range failures {
		meta, _ := lstatOrZero(failure.path)
		row := metaRow(owner, failure.path, "dir", meta)
		row.Size, row.Status, row.Error = 0, manifest.ErrorRead, failure.reason
		if collector.Owners != nil && isVisibleDir(failure.path) {
			row.Owner = collector.ownerOf(failure.path)
		}
		if err := collector.Manifest.Write(row); err != nil {
			return err
		}
		if err := collector.logReadError(failure.path, failure.reason); err != nil {
			return err
		}
	}
	return nil
}

// logReadError logs the read failed: line of a directory that cannot be
// listed or entered.
func (collector *Collector) logReadError(path, reason string) error {
	return collector.Log.Line("read failed: " + syspath.ToNative(path) + ": " + reason)
}

// recordFiles gives each regular file's size and modification time to the
// inventory records, as INVENTORY_FILES_WORKER prints them. A path lstat
// describes as anything else (a reparse point Windows lists as a regular
// file) is not counted, as Add-CollectedFile counts only type file.
func (collector *Collector) recordFiles(owner scope, paths []string) {
	for _, path := range paths {
		meta, exists := lstatOrZero(path)
		if exists && meta.Kind != fsmeta.File {
			continue
		}
		collector.Records.File(owner.agent, max(meta.Size, 0), max(meta.Mtime, 0))
	}
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

// lstatOrZero returns path's lstat and true, or fsmeta.ZeroMeta, the
// metadata of a path that cannot be described, and false.
func lstatOrZero(path string) (fsmeta.Meta, bool) {
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return fsmeta.ZeroMeta(), false
	}
	return meta, true
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
