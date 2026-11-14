//go:build !darwin

package walk

// findRunsLastBatchFirst is false where find runs its pending batched -exec
// programs in expression order when a walk ends, as GNU find does: a walk's
// skipped_excluded rows, then its error_read rows, then its file rows.
const findRunsLastBatchFirst = false
