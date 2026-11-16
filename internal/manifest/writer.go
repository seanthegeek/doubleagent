package manifest

import (
	"io"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Counts is the number of rows per status and the bytes the collected rows
// hold, the figures collection.json reports.
type Counts struct {
	ByStatus       map[Status]int
	CollectedBytes int64
}

// Writer writes rows to an io.Writer, one write per row, and remembers what
// the summary needs: the counts, the paths written and the homes that have a
// row, each path once per file system entry (syspath.Entries). It is not
// safe for use by several goroutines.
type Writer struct {
	out    io.Writer
	counts Counts
	same   syspath.SameEntry
	paths  *syspath.Entries[struct{}]
	homes  *syspath.Entries[struct{}]
	line   []byte
}

// NewWriter returns a Writer that appends rows to out. Until UseSameEntry
// is called, two spellings of a path are two entries unless their bytes are
// equal.
func NewWriter(out io.Writer) *Writer {
	writer := &Writer{out: out, counts: Counts{ByStatus: map[Status]int{}}}
	same := func(first, second string) bool {
		return writer.same != nil && writer.same(first, second)
	}
	writer.paths = syspath.NewEntries[struct{}](same)
	writer.homes = syspath.NewEntries[struct{}](same)
	return writer
}

// UseSameEntry sets the question the writer asks the file system about two
// spellings of a path whose syspath.Fold keys are equal; the writer does no
// file system call of its own.
func (writer *Writer) UseSameEntry(same syspath.SameEntry) {
	writer.same = same
}

// Write appends row as one line and counts it. A row is counted only when the
// write succeeds.
func (writer *Writer) Write(row Row) error {
	// The canonical forms are the keys; the written forms are native.
	path, home := row.Path, row.Home
	ownRow := path == home && row.ArchivePath == "" && row.Type == "dir"
	row.Path = syspath.ToNative(row.Path)
	row.Home = syspath.ToNative(row.Home)
	writer.line = appendRow(writer.line[:0], row)
	if _, err := writer.out.Write(writer.line); err != nil {
		return err
	}
	writer.counts.ByStatus[row.Status]++
	if row.Status == Collected {
		writer.counts.CollectedBytes += nonNegative(row.Size)
	}
	writer.paths.Put(path, struct{}{})
	// A home's own unreadable-directory row does not make it active.
	if home != "" && !ownRow {
		writer.homes.Put(home, struct{}{})
	}
	return nil
}

// Counts returns the rows written per status and the collected bytes.
func (writer *Writer) Counts() Counts {
	byStatus := make(map[Status]int, len(writer.counts.ByStatus))
	for status, count := range writer.counts.ByStatus {
		byStatus[status] = count
	}
	return Counts{ByStatus: byStatus, CollectedBytes: writer.counts.CollectedBytes}
}

// HasPath reports whether a row was written for the file system entry the
// canonical path names: one with the same bytes, or on Windows one spelled
// otherwise in ASCII case that the file system says is the same entry. In a
// case-sensitive directory Alice and alice are two entries.
func (writer *Writer) HasPath(path string) bool {
	return writer.paths.Has(path)
}

// HasHomeRows reports whether home has a row other than its own
// unreadable-directory row, its entry found as HasPath finds a path.
func (writer *Writer) HasHomeRows(home string) bool {
	return writer.homes.Has(home)
}

// nonNegative returns value, or 0 for a negative value: the sh collector writes
// 0 for any number that is not a run of digits, and a time before 1970 is one.
func nonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

// appendString appends `"key":"escaped value"` and the separator.
func appendString(out []byte, key, value, separator string) []byte {
	out = append(out, '"')
	out = append(out, key...)
	out = append(out, `":"`...)
	out = appendEscaped(out, value)
	out = append(out, '"')
	return append(out, separator...)
}

// appendNumber appends `"key":value` and the separator.
func appendNumber(out []byte, key string, value int64, separator string) []byte {
	out = append(out, '"')
	out = append(out, key...)
	out = append(out, `":`...)
	out = strconv.AppendInt(out, nonNegative(value), 10)
	return append(out, separator...)
}

// appendRow appends the line for row, with no spaces, numbers and the secret
// flag unquoted, and a final newline.
func appendRow(out []byte, row Row) []byte {
	out = append(out, '{')
	out = appendString(out, "user", row.User, ",")
	out = appendString(out, "home", row.Home, ",")
	out = appendString(out, "agent", row.Agent, ",")
	out = appendString(out, "path", row.Path, ",")
	out = appendString(out, "archive_path", row.ArchivePath, ",")
	out = appendString(out, "type", row.Type, ",")
	out = appendNumber(out, "size", row.Size, ",")
	out = appendNumber(out, "mtime", row.Mtime, ",")
	out = appendNumber(out, "atime", row.Atime, ",")
	out = appendNumber(out, "ctime", row.Ctime, ",")
	out = appendNumber(out, "btime", row.Btime, ",")
	out = appendNumber(out, "uid", int64(row.UID), ",")
	out = appendNumber(out, "gid", int64(row.GID), ",")
	out = appendString(out, "mode", row.Mode, ",")
	out = appendPlatformFields(out, row)
	out = appendString(out, "sha256", row.SHA256, ",")
	out = append(out, `"secret":`...)
	out = strconv.AppendBool(out, row.Secret)
	out = append(out, ',')
	out = appendString(out, "status", string(row.Status), ",")
	out = appendString(out, "target", row.Target, ",")
	out = appendString(out, "error", row.Error, "")
	return append(out, "}\n"...)
}

// appendWindowsFields appends `"owner":"...","attributes":"...",`, the two
// fields the PowerShell collector writes between mode and sha256.
func appendWindowsFields(out []byte, row Row) []byte {
	out = appendString(out, "owner", row.Owner, ",")
	return appendString(out, "attributes", row.Attributes, ",")
}
