package walk

// findRunsLastBatchFirst is true where find runs its pending batched -exec
// programs in the reverse of their order in the expression when a walk ends:
// the BSD find of macOS writes a walk's file rows, then its error_read rows,
// then its skipped_excluded rows.
const findRunsLastBatchFirst = true

// findReportsUnenterableEntries is false where find writes nothing for a
// directory below its start that can be listed but not entered: the BSD find
// of macOS neither lists its entries nor reports them.
const findReportsUnenterableEntries = false
