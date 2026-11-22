package walk

// findRunsLastBatchFirst is unused on Windows, where no find runs.
const findRunsLastBatchFirst = false

// rowsFollowWalk is true on Windows: each path's row is written as the walk
// reaches it, so a walk's rows, and the log lines of their failed copies and
// links, follow the walk.
const rowsFollowWalk = true
