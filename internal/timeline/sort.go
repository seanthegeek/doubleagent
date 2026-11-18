package timeline

import (
	"cmp"
	"slices"
	"strings"
)

// SortRows sorts rows in place, stably, into timeline order: dated rows
// first by timestamp, then undated ones, each by source file and line, rows
// equal in all of these keeping their order.
func SortRows(rows []Row) {
	slices.SortStableFunc(rows, CompareRows)
}

// CompareRows orders two rows as SortRows does, returning a negative
// number, zero or a positive number.
func CompareRows(left, right Row) int {
	leftUndated, rightUndated := left.TimestampUTC == "", right.TimestampUTC == ""
	switch {
	case leftUndated && !rightUndated:
		return 1
	case rightUndated && !leftUndated:
		return -1
	}
	return cmp.Or(strings.Compare(left.TimestampUTC, right.TimestampUTC),
		strings.Compare(left.SourceFile, right.SourceFile),
		cmp.Compare(left.SourceLine, right.SourceLine))
}
