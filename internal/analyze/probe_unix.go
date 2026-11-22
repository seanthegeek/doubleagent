//go:build unix

package analyze

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// probeWriteThen is probeWrite with a call, when set, between the probe's
// creation and its removal, given the probe's name. The directory is opened
// once and both are done below that handle, so a directory moved aside in
// between and a link put at its name change nothing removed.
func probeWriteThen(directory string, created func(probe string)) bool {
	root, err := osfile.OpenRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	random := make([]byte, 16)
	_, _ = rand.Read(random)
	probe := ".write-probe-" + hex.EncodeToString(random)
	file, err := root.OpenFile(probe, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false
	}
	_ = file.Close()
	if created != nil {
		created(probe)
	}
	_ = root.Remove(probe)
	return true
}
