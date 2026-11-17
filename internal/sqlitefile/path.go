package sqlitefile

import (
	"errors"
	"fmt"
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
// skipped, as a copy of regular files would leave it out.
func OpenPath(path string) (*Database, error) {
	var opened []io.Closer
	files := Files{}
	targets := []struct {
		suffix string
		slot   **io.SectionReader
	}{{"", &files.Main}, {"-wal", &files.Log}, {"-journal", &files.Journal}}
	for _, target := range targets {
		file, section, err := openRegular(path+target.suffix, target.suffix == "")
		if err != nil {
			return nil, errors.Join(err, closeAll(opened))
		}
		if file != nil {
			opened = append(opened, file)
			*target.slot = section
		}
	}
	database, err := Open(files)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open %s: %w", path, err), closeAll(opened))
	}
	database.closers = opened
	return database, nil
}

// openRegular opens path when its own entry is a regular file and returns
// the file and a reader of its length. An absent sidecar, or one that is
// not a regular file or a link, gives nil and no error; for the main file
// every such case is an error. The opened file must be the entry the lstat
// described, so a link swapped in between is refused.
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
		return nil, nil, fmt.Errorf("%s is a symbolic link, not opened", path)
	case !info.Mode().IsRegular() && required:
		return nil, nil, fmt.Errorf("%s is not a regular file", path)
	case !info.Mode().IsRegular():
		return nil, nil, nil
	}
	file, err := osfile.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err == nil && !os.SameFile(info, opened) {
		err = fmt.Errorf("%s changed while it was opened", path)
	}
	if err != nil {
		return nil, nil, errors.Join(err, file.Close())
	}
	return file, io.NewSectionReader(file, 0, opened.Size()), nil
}

// closeAll closes every file and joins their errors.
func closeAll(files []io.Closer) error {
	var errs []error
	for _, file := range files {
		errs = append(errs, file.Close())
	}
	return errors.Join(errs...)
}
