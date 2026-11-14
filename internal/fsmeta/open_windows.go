package fsmeta

import "syscall"

// openRegularFlags make OpenRegular's open take a reparse point (a symlink,
// junction or socket) itself rather than follow it, so the handle check that
// follows refuses one swapped in after the lstat. A Windows file system
// holds no named pipe an open could wait on, so no non-blocking flag is
// needed.
const openRegularFlags = syscall.FILE_FLAG_OPEN_REPARSE_POINT
