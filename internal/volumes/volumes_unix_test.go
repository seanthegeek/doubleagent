//go:build unix

package volumes

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// These need POSIX permission bits to make a directory unreadable.
func TestUnreadableVolumeIsNotedAndCounted(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/ollama/_data/f", "x")
	volume := filepath.Join(fixture.root, "var/lib/docker/volumes/ollama")
	if err := os.Chmod(volume, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(volume, 0o755) })
	var notes []string
	got, err := Collect(context.Background(), fixture.collector, fixture.root, nil, &notes)
	if err != nil {
		t.Fatal(err)
	}
	want := "docker: volume ollama (ollama) at " + volume + " is not readable; run as root to collect it"
	if got.Unreadable != 1 || got.Collected != 0 || len(notes) != 1 || notes[0] != want {
		t.Errorf("counts = %+v, notes = %q", got, notes)
	}
}
