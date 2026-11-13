//go:build !darwin

package walk

// findRunsLastBatchFirst is false where find runs its pending batched -exec
// programs in expression order when a walk ends, as GNU find does: a walk's
// skipped_excluded rows, then its error_read rows, then its file rows.
const findRunsLastBatchFirst = false

// findReportsUnenterableEntries is true where find lists a directory that
// can be listed but not entered and writes one error line per entry it
// cannot stat there, as GNU find does; the edge-tree oracle test compares
// the walk's lines with the platform's find.
const findReportsUnenterableEntries = true
