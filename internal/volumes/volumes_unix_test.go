//go:build unix

package volumes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnreadableVolumeIsNotedAndCounted needs POSIX permission bits to make a
// directory unreadable; TestUnmatchedVolumeRow expects the paths as the host
// spells them, with slashes only.
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

func TestUnmatchedVolumeRow(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/PG_VERSION", "16")
	var notes []string
	if _, err := Collect(context.Background(), fixture.collector, fixture.root, nil, &notes); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(fixture.root, "var/lib/docker/volumes/postgres/_data")
	want := `{"user":"docker","home":"` + data + `","agent":"","path":"` + data +
		`","archive_path":"","type":"dir","size":`
	row := fixture.rows.String()
	if !strings.HasPrefix(row, want) ||
		!strings.HasSuffix(row, `,"mtime":0,"atime":0,"ctime":0,"btime":0,"uid":0,"gid":0,`+
			`"mode":"0","sha256":"","secret":false,"status":"skipped_unmatched_volume",`+
			`"target":"","error":""}`+"\n") {
		t.Errorf("row = %s", row)
	}
}
