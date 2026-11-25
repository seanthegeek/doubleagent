//go:build windows

package main

import "os"

// pipeSignals is none on Windows, where a write to a pipe whose reader
// ended fails without a signal.
func pipeSignals() []os.Signal { return nil }
