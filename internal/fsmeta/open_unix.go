//go:build unix

package fsmeta

import "syscall"

// openRegularFlags make OpenRegular's open fail on a symlink at the last
// component rather than follow it, and return at once on a pipe with no
// writer rather than wait for one.
const openRegularFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
