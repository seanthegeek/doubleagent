//go:build unix

package collect

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// The environment variables that give TestSignalHelperProcess its image root
// and output directory.
const (
	SignalHelperRoot   = "DOUBLEAGENT_SIGNAL_HELPER_ROOT"
	SignalHelperOutDir = "DOUBLEAGENT_SIGNAL_HELPER_OUT"
)

// TestSignalHelperProcess is the program the signal tests start, not a test
// of its own. It runs a collection of the image root its environment names,
// with Run's host and signal handlers, and pauses once the homes are found:
// it writes a line to file descriptor 4, then waits until the run is told to
// stop or descriptor 3 is closed, so a signal sent during the pause arrives
// mid-run however fast the machine is. It exits with the run's exit code.
func TestSignalHelperProcess(t *testing.T) {
	root := os.Getenv(SignalHelperRoot)
	if root == "" {
		t.Skip("run only by the signal tests, as their program")
	}
	release, paused := os.NewFile(3, "release"), os.NewFile(4, "paused")
	released := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, release)
		close(released)
	}()
	host := systemHost()
	find := host.findUsers
	host.findUsers = func(ctx context.Context, live bool, root, usersFilter string, run users.Runner,
		check glob.Check, opener *fsmeta.Opener, respell func(string) string,
	) users.Report {
		report := find(ctx, live, root, usersFilter, run, check, opener, respell)
		_, _ = io.WriteString(paused, "paused\n")
		select {
		case <-ctx.Done():
		case <-released:
		}
		return report
	}
	ctx, stop := catchSignals(context.Background())
	code := runCollection(ctx, testOptions(root, os.Getenv(SignalHelperOutDir)), host, io.Discard, io.Discard)
	stop()
	os.Exit(code)
}
