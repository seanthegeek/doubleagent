package walk

// findRunsLastBatchFirst is true where find runs its pending batched -exec
// programs in the reverse of their order in the expression when a walk ends:
// the BSD find of macOS writes a walk's file rows, then its error_read rows,
// then its skipped_excluded rows.
const findRunsLastBatchFirst = true
