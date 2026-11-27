package sqlitefile

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// OpenPath opens the database at path with its path-wal and path-journal
// files, when they are present, and reads them in place; Close releases the
// files. It never writes, so SQLite's own leftovers stay as they are. A
// symbolic link is refused, never followed, for the database and its
// sidecars alike; a sidecar that is a directory or another special file is
// skipped, as a copy of regular files would leave it out. No error names
// the directory, since a parser shows it in a row: a file that cannot be
// opened or read is an *UnreadableError, and a damaged database gives
// openFiles's error alone, the caller knowing which file it asked for.
func OpenPath(path string) (*Database, error) {
	var opened []io.Closer
	files := databaseFiles{}
	targets := []struct {
		suffix string
		slot   **io.SectionReader
	}{{"", &files.Main}, {"-wal", &files.Log}, {"-journal", &files.Journal}}
	for _, target := range targets {
		file, section, err := openRegular(path+target.suffix, target.suffix == "")
		if err != nil {
			unreadable := &UnreadableError{Path: path + target.suffix, Err: err}
			return nil, errors.Join(unreadable, closeAll(opened))
		}
		if file != nil {
			opened = append(opened, file)
			*target.slot = section
		}
	}
	database, err := openFiles(files)
	if err != nil {
		return nil, errors.Join(err, closeAll(opened))
	}
	database.closers = opened
	return database, nil
}

// openRegular opens path when its own entry is a regular file and returns
// the file and a reader of its length. An absent sidecar, or one that is
// not a regular file or a link, gives nil and no error; for the main file
// every such case is an error. The opened file must be the entry the lstat
// described, so a link swapped in between is refused. The reader reports a
// failed read as an *UnreadableError, so the path stays out of its text.
func openRegular(path string, required bool) (*os.File, *io.SectionReader, error) {
	info, err := osfile.Lstat(path)
	if !required && errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return nil, nil, errSymbolicLink
	case !info.Mode().IsRegular() && required:
		return nil, nil, errNotRegular
	case !info.Mode().IsRegular():
		return nil, nil, nil
	}
	file, err := osfile.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err == nil && !os.SameFile(info, opened) {
		err = errChanged
	}
	if err != nil {
		return nil, nil, errors.Join(err, file.Close())
	}
	reader := quietReader{file: file, path: path}
	return file, io.NewSectionReader(reader, 0, opened.Size()), nil
}

// errChanged is the reason a file replaced between its lstat and its open
// is not read.
var errChanged = errors.New("changed while it was opened")

// quietReader reads a file and reports a failed read as an *UnreadableError
// in place of the *fs.PathError that would name the file's full path.
type quietReader struct {
	file *os.File
	path string
}

// ReadAt reads from the file, rewording a failure; io.EOF stays as it is.
func (r quietReader) ReadAt(target []byte, offset int64) (int, error) {
	n, err := r.file.ReadAt(target, offset)
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		err = &UnreadableError{Path: r.path, Err: pathError.Err}
	}
	return n, err
}

// closeAll closes every file and joins their errors.
func closeAll(files []io.Closer) error {
	var errs []error
	for _, file := range files {
		errs = append(errs, file.Close())
	}
	return errors.Join(errs...)
}
