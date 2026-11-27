package sqlitefile

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// OpenPath opens the database at path with its path-wal and path-journal
// files, when they are present, and reads them in place; Close releases the
// files. It never writes, so SQLite's own leftovers stay as they are. A
// symbolic link is refused, never followed, for the database and its
// sidecars alike; a sidecar that is a directory or another special file is
// skipped, as a copy of regular files would leave it out, and one absent as
// built is taken as listing, the listing of a directory, spells it
// (openTarget). No error names the directory, since a parser shows it in a
// row: a file that cannot be opened or read is an *UnreadableError, and a
// damaged database gives openFiles's error alone.
func OpenPath(path string, listing func(directory string) osfile.Spellings) (*Database, error) {
	var opened []io.Closer
	files := databaseFiles{}
	targets := []struct {
		suffix string
		slot   **io.SectionReader
	}{{"", &files.Main}, {"-wal", &files.Log}, {"-journal", &files.Journal}}
	for _, target := range targets {
		name, file, section, err := openTarget(path, target.suffix, listing)
		if err != nil {
			unreadable := &UnreadableError{Path: name, Err: err}
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

// openTarget opens path+suffix, the database itself when suffix is empty,
// else a sidecar, and returns the path opened with the file and a reader
// as openRegular gives them. A sidecar absent as built is taken as the
// listing of the database's directory spells it (osfile.Spellings); absent
// in every case, it gives nil and no error.
func openTarget(path, suffix string, listing func(directory string) osfile.Spellings,
) (string, *os.File, *io.SectionReader, error) {
	name := path + suffix
	file, section, err := openRegular(name, suffix == "")
	if errors.Is(err, errAbsent) {
		directory, base := filepath.Dir(name), filepath.Base(name)
		if spelled := listing(directory).Spelling(base); spelled != base {
			name = filepath.Join(directory, spelled)
			file, section, err = openRegular(name, false)
		}
	}
	if errors.Is(err, errAbsent) {
		err = nil
	}
	return name, file, section, err
}

// errAbsent is a sidecar that does not exist, which is not an error.
var errAbsent = errors.New("absent")

// openRegular opens path when its own entry is a regular file and returns
// the file and a reader of its length, by OpenPath's rules for the main
// file (required) or a sidecar; an absent sidecar gives errAbsent, and a
// skipped one nil and no error. The opened file must be the entry the
// lstat described, so a link swapped in between is refused.
func openRegular(path string, required bool) (*os.File, *io.SectionReader, error) {
	info, err := osfile.Lstat(path)
	if !required && errors.Is(err, fs.ErrNotExist) {
		return nil, nil, errAbsent
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
