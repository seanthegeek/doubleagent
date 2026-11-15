package fsmeta

// openRegularFlags is 0: Windows has no O_NOFOLLOW or O_NONBLOCK, and
// OpenRegular's Lstat has already refused a symlink.
const openRegularFlags = 0
