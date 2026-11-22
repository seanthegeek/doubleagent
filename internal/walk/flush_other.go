//go:build !darwin && !windows

package walk

// fileRowsFirst is false on Linux and the other Unix systems, where a walk's
// rows come in three groups when the walk ends: its skipped_excluded rows,
// then its error_read rows, then its file rows.
const fileRowsFirst = false

// rowsFollowWalk is false where a walk's rows are written grouped by kind,
// in the order fileRowsFirst gives.
const rowsFollowWalk = false
