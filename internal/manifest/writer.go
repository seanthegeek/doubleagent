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
	same    syspath.SameEntry
	paths   *syspath.Entries[struct{}]
	homes   *syspath.Entries[struct{}]
	encoder *json.Encoder
}

// NewWriter returns a Writer that appends rows to out. Until UseSameEntry
// is called, two spellings of a path are two entries unless their bytes are
// equal.
func NewWriter(out io.Writer) *Writer {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	writer := &Writer{encoder: encoder, counts: Counts{ByStatus: map[Status]int{}}}
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

// Write appends row as one line, one write to the output, and counts it. A row is counted only when the
// write succeeds.
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
