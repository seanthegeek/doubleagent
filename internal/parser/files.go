package parser

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The reasons a path that is not a regular file is not opened.
var (
	errSymbolicLink = errors.New("a symbolic link, not followed")
	errNotRegular   = errors.New("not a regular file")
	errChanged      = errors.New("changed while it was opened")
)

// byteOrderMark is U+FEFF in UTF-8, which some agents write at the start of
// a JSON file.
const byteOrderMark = "\ufeff"

// entryError returns why the entry info describes is not opened as a file,
// or nil for a regular file: a link is never followed, and a directory, a
// named pipe or a device is never opened.
func entryError(path string, info fs.FileInfo) error {
	reason := error(nil)
	switch mode := info.Mode(); {
	case mode&fs.ModeSymlink != 0:
		reason = errSymbolicLink
	case mode.IsDir():
		reason = syscall.EISDIR
	case !mode.IsRegular():
		reason = errNotRegular
	}
	if reason == nil {
		return nil
	}
	return &fs.PathError{Op: "open", Path: path, Err: reason}
}

// Open opens the regular file at path for reading. A symbolic link is never
// followed and a directory, a named pipe or a device is never opened: each
// is an error, as is a file replaced between the look at its entry and the
// open. The open itself follows no link at the last element and does not
// wait on a named pipe (on Unix O_NOFOLLOW and O_NONBLOCK; on Windows the
// handle is opened on the entry's own reparse point, and a named pipe is
// not a file of a directory there), and the handle must be the regular file
// the look described. The directories above the last element are not
// looked at here: Beside, Above, Under and Home.PathOf, through which a
// parser builds every path it opens, refuse a link on the way down.
func Open(path string) (*os.File, error) {
	info, err := osfile.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := entryError(path, info); err != nil {
		return nil, err
	}
	return openLooked(path, info)
}

// openLooked opens path, whose entry an lstat described as info, and
// returns the file when the handle is that regular file.
func openLooked(path string, info fs.FileInfo) (*os.File, error) {
	file, err := (&fsmeta.Opener{ReadOnly: true}).Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err == nil && (!opened.Mode().IsRegular() || !os.SameFile(info, opened)) {
		err = &fs.PathError{Op: "open", Path: path, Err: errChanged}
	}
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

// ReadFile returns the whole content of the regular file at path, opened as
// Open opens it.
func ReadFile(path string) ([]byte, error) {
	file, err := Open(path)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// FileBytes returns the content of the file at path, or false after adding
// its unreadable problem, at line 0, to problems.
func FileBytes(path string, problems *Problems) ([]byte, bool) {
	data, err := ReadFile(path)
	if err != nil {
		problems.AddUnreadable(0, path, err)
		return nil, false
	}
	return data, true
}

// RegularFile reports whether path's own entry is a regular file; a link,
// whatever it points at, is not.
func RegularFile(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

// IsLink reports whether path's own entry is a symbolic link; a path that
// cannot be looked at is not one.
func IsLink(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}

// IsDirectory reports whether path's own entry is a directory, not a link
// to one.
func IsDirectory(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.IsDir()
}

// ReadText returns the text of an optional side file, decoded as UTF-8 with
// replacement and without a leading byte-order mark. A path that is absent,
// a link or anything but a regular file gives false silently, since a side
// file is optional; a regular file that cannot be read gives false after
// adding its unreadable problem to problems.
func ReadText(path string, problems *Problems) (string, bool) {
	if !RegularFile(path) {
		return "", false
	}
	data, ok := FileBytes(path, problems)
	if !ok {
		return "", false
	}
	return strings.TrimPrefix(textlines.ValidUTF8(data), byteOrderMark), true
}

// LoadJSON returns an optional side file decoded as JSON, read as ReadText
// reads it. Only a file that cannot be read adds a problem; for text that is
// not JSON the caller writes its own message, so it gives false silently.
func LoadJSON(path string, problems *Problems) (jsonvalue.Value, bool) {
	text, ok := ReadText(path, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		return jsonvalue.NewNull(), false
	}
	return value, true
}

// ReadDir lists the directory at path sorted by name as bytes, not in the
// order the file system gives, so a parser that reads files in listing
// order reads them in one order on every file system. It does not follow a
// link at path itself; the entries are never followed either.
func ReadDir(path string) ([]os.DirEntry, error) {
	info, err := osfile.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		reason := error(syscall.ENOTDIR)
		if info.Mode()&fs.ModeSymlink != 0 {
			reason = errSymbolicLink
		}
		return nil, &fs.PathError{Op: "readdir", Path: path, Err: reason}
	}
	entries, err := osfile.ReadDir(path)
	slices.SortFunc(entries, func(first, second os.DirEntry) int {
		return strings.Compare(first.Name(), second.Name())
	})
	return entries, err
}

// FileMtime returns the artifact's modification time as a timeline
// timestamp: the manifest's pre-copy mtime when it is a positive number,
// else the file's own, else "". Rows for standing settings carry it.
func FileMtime(artifact *input.Artifact) string {
	mtime := artifact.Mtime()
	// A positive number beyond the years a timestamp holds gives no
	// timestamp rather than the file's own.
	if mtime.IsNumber() && !mtime.IsEmpty() && !strings.HasPrefix(string(mtime.Number()), "-") {
		return timeutil.FromValue(mtime)
	}
	info, err := osfile.Lstat(artifact.DiskPath)
	if err != nil {
		return ""
	}
	return timeutil.FormatUTC(info.ModTime())
}

// MtimeSeconds returns the modification time of an entry already looked
// at, in seconds since the epoch as a float, for a parser that compares or
// converts the times of several files.
func MtimeSeconds(info fs.FileInfo) float64 {
	return statSeconds(info.ModTime().Unix(), info.ModTime().Nanosecond())
}

// statSeconds returns a modification time in seconds: seconds plus
// nanoseconds times 1e-9, each step rounded to a float64, so the sum is the
// same double on every platform and never a fused multiply-add.
func statSeconds(seconds int64, nanoseconds int) float64 {
	fraction := float64(float64(nanoseconds) * 1e-9)
	return float64(seconds) + fraction
}
