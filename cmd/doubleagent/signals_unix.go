//go:build unix

package main

import (
	"os"
	"syscall"
)

// pipeSignals is SIGPIPE, sent for a write to a pipe whose reader ended;
// caught, the write fails with EPIPE instead of ending the program
// (os/signal's SIGPIPE section).
func pipeSignals() []os.Signal { return []os.Signal{syscall.SIGPIPE} }
