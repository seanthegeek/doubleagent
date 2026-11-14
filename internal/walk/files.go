package walk

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/manifest"
)

// plannedFile is one file through the staging phases: its lstat before the
// copy, the outcome planned (an empty status means the file is to be copied)
// and what the copy gave.
type plannedFile struct {
	path      string
	meta      fsmeta.Meta
	kind      string
	status    manifest.Status
	secret    bool
	target    string
	sha256    string
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
	file := plannedFile{path: path, meta: meta, kind: "file"}
	file.secret = secret != nil && secret(strings.TrimPrefix(path, owner.home+"/"))
	switch {
	case exists && meta.Kind == fsmeta.Symlink:
		file.status, file.kind = manifest.Symlink, "symlink"
		file.target, _ = os.Readlink(path)
		_ = owner.stager.Symlink(path, file.target)
	case exists && meta.Kind != fsmeta.File:
		file.status, file.kind = manifest.SkippedSpecial, specialKind(meta.Kind)
	case file.secret && collector.NoSecrets:
		file.status = manifest.SkippedSecret
	case collector.MaxSize > 0 && meta.Size > collector.MaxSize:
		file.status = manifest.SkippedSize
	}
	return file
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

// transferFiles copies every file planned for a copy and records the hash or
// the error. A failed copy's partial file is removed when it lies inside the
// staging directory, and the log line says when it was not.
func (collector *Collector) transferFiles(owner scope, files []plannedFile) error {
	for i := range files {
		file := &files[i]
		if file.status != "" {
			continue
		}
		hash, err := owner.stager.CopyFile(file.path)
		if err == nil {
			file.status, file.sha256 = manifest.Collected, hash
			continue
		}
		file.status, file.copyError = manifest.ErrorCopy, innermostError(err)
		note := ""
		if _, dest := owner.stager.Dest(file.path); !owner.stager.RemoveFailedCopy(dest) {
			note = "; staged path resolves outside the staging directory, not removed"
		}
		line := "copy failed: " + file.path + ": " + file.copyError + note
		if err := collector.Log.Line(line); err != nil {
			return err
		}
	}
	return nil
}

// fileRow is the manifest row of one staged file.
func fileRow(owner scope, file plannedFile) manifest.Row {
	row := metaRow(owner, file.path, file.kind, file.meta)
	row.Secret, row.Status, row.Target = file.secret, file.status, file.target
	row.SHA256, row.Error = file.sha256, file.copyError
	if file.status == manifest.Collected || file.status == manifest.Symlink {
		row.ArchivePath, _ = owner.stager.Dest(file.path)
	}
	return row
}

// writeExcluded writes one skipped_excluded row per path, as
// EXCLUDED_PATHS_WORKER does: a directory, or a link to one, as a dir row
// sized as du -sk sizes it with its other metadata 0, anything else as a
// file row with its lstat. It stops with ctx's error when ctx is cancelled
// while a directory is being sized.
func (collector *Collector) writeExcluded(ctx context.Context, owner scope, paths []string,
) error {
	for _, path := range paths {
		var row manifest.Row
		if pointsToDir(path) {
			size, err := fsmeta.DiskUsageBytes(ctx, path)
			if err != nil {
				return err
			}
			row = metaRow(owner, path, "dir", zeroMeta())
			row.Size = size
		} else {
			meta, _ := lstatOrZero(path)
			row = metaRow(owner, path, "file", meta)
		}
		row.Status = manifest.SkippedExcluded
		if err := collector.Manifest.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// writeReadErrors writes one error_read row per directory, with its lstat
// metadata and size 0, and its log line, as write_read_error_row does.
func (collector *Collector) writeReadErrors(owner scope, failures []readError) error {
	for _, failure := range failures {
		meta, _ := lstatOrZero(failure.path)
		row := metaRow(owner, failure.path, "dir", meta)
		row.Size, row.Status, row.Error = 0, manifest.ErrorRead, failure.reason
		if err := collector.Manifest.Write(row); err != nil {
			return err
		}
		line := "read failed: " + failure.path + ": " + failure.reason
		if err := collector.Log.Line(line); err != nil {
			return err
		}
	}
	return nil
}

// recordFiles gives each path's size and modification time to the inventory
// records, as INVENTORY_FILES_WORKER prints them.
func (collector *Collector) recordFiles(owner scope, paths []string) {
	for _, path := range paths {
		meta, _ := lstatOrZero(path)
		collector.Records.File(owner.agent, max(meta.Size, 0), max(meta.Mtime, 0))
	}
}

// metaRow is a row of owner for path with the metadata of meta.
func metaRow(owner scope, path, kind string, meta fsmeta.Meta) manifest.Row {
	return manifest.Row{
		User: owner.user, Home: owner.home, Agent: owner.agent, Path: path, Type: kind,
		Size: meta.Size, Mtime: meta.Mtime, Atime: meta.Atime, Ctime: meta.Ctime,
		Btime: meta.Btime, UID: meta.UID, GID: meta.GID, Mode: meta.Mode,
	}
}

// lstatOrZero returns path's lstat and true, or the all-zero metadata the sh
// collector writes when stat prints nothing, with mode "0", and false.
func lstatOrZero(path string) (fsmeta.Meta, bool) {
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return zeroMeta(), false
	}
	return meta, true
}

// zeroMeta is the metadata of the sh collector's "0 0 0 0 0 0 0 0" stat line.
func zeroMeta() fsmeta.Meta {
	return fsmeta.Meta{Mode: "0"}
}

// pointsToDir reports whether path is a directory or a symlink to one, the
// test(1) -d the excluded-path worker applies. Only the type is read, so
// following the link reads nothing through it.
func pointsToDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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
