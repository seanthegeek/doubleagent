package walk

// fileRowsFirst is true on macOS, where a walk's rows come in three groups
// when the walk ends: its file rows, then its error_read rows, then its
// skipped_excluded rows.
const fileRowsFirst = true

// rowsFollowWalk is false where a walk's rows are written grouped by kind,
// in the order fileRowsFirst gives.
const rowsFollowWalk = false
