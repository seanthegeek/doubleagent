package manifest

import (
	"encoding/json"
	"io"

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
	counts  Counts
	paths   *syspath.Entries[struct{}]
	homes   *syspath.Entries[struct{}]
	encoder *json.Encoder
}

// NewWriter returns a Writer that appends rows to out. same decides whether
// two spellings with equal syspath.Fold keys are one entry (nil: only equal
// bytes are); the writer makes no file system call of its own.
func NewWriter(out io.Writer, same syspath.SameEntry) *Writer {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	return &Writer{encoder: encoder, counts: Counts{ByStatus: map[Status]int{}},
		paths: syspath.NewEntries[struct{}](same), homes: syspath.NewEntries[struct{}](same)}
}

// Write appends row as one line, in one write, and counts it once the write
// succeeds.
func (writer *Writer) Write(row Row) error {
	// The canonical forms are the keys; the written forms are native.
	path, home := row.Path, row.Home
	ownRow := path == home && row.ArchivePath == "" && row.Type == "dir"
	row.Path = syspath.ToNative(row.Path)
	row.Home = syspath.ToNative(row.Home)
	if err := writer.encoder.Encode(jsonRowOf(row)); err != nil {
		return err
	}
	writer.counts.ByStatus[row.Status]++
	if row.Status == Collected {
		// A negative size counts as 0, as jsonRowOf writes it.
		writer.counts.CollectedBytes += max(row.Size, 0)
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
// canonical path names: the same bytes, or on Windows another ASCII case
// that the file system says is the same entry (in a case-sensitive directory
// Alice and alice are two).
func (writer *Writer) HasPath(path string) bool {
	return writer.paths.Has(path)
}

// HasHomeRows reports whether home has a row other than its own
// unreadable-directory row, its entry found as HasPath finds a path.
func (writer *Writer) HasHomeRows(home string) bool {
	return writer.homes.Has(home)
}
