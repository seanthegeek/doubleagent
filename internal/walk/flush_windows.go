package walk

// findRunsLastBatchFirst is unused on Windows, where no find runs.
const findRunsLastBatchFirst = false

// rowsFollowWalk is true on Windows: the PowerShell collector's
// Add-CollectedTree writes each path's row as it reaches it, so a walk's
// rows, and the log lines of their failed copies and links, follow the walk.
const rowsFollowWalk = true
