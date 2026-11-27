//go:build unix

package volumes

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/users"
)

// imageFixture writes the image the volume test collects: matched, unmatched
// and empty volumes, a rootless volume of a home, a symlinked volumes
// directory and two Docker Desktop directories.
func imageFixture(t *testing.T, fixture setup) {
	t.Helper()
	fixture.write(t, "etc/passwd", "alice:x:1000:1000::/home/alice:/bin/sh\n")
	fixture.mkdir(t, "home/alice")
	fixture.write(t, "var/lib/docker/volumes/a0_usr/_data/x/file", "hi")
	fixture.write(t, "var/lib/docker/volumes/a0_usr/_data/tmp/playwright/cache", "junk")
	fixture.write(t, "var/lib/docker/volumes/a0_usr/_data/secrets.env", "KEY=1")
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/PG_VERSION", "16")
	fixture.write(t, "var/lib/docker/volumes/ollama_data/_data/models/blob", "m")
	fixture.mkdir(t, "var/lib/docker/volumes/langflow-data")
	fixture.mkdir(t, "var/lib/docker/volumes/n8n_data/other")
	fixture.write(t, "home/alice/.local/share/docker/volumes/hermes1/_data/f", "x")
	fixture.mkdir(t, "elsewhere")
	fixture.mkdir(t, "var/lib/containers/storage")
	err := os.Symlink(filepath.Join(fixture.root, "elsewhere"),
		filepath.Join(fixture.root, "var/lib/containers/storage/volumes"))
	if symlinkNeedsPrivilege(err) {
		t.Skipf("creating a symbolic link needs the symbolic-link privilege: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	fixture.mkdir(t, "ProgramData/DockerDesktop")
	fixture.mkdir(t, "home/alice/Library/Containers/com.docker.docker")
}

var (
	timeFields  = regexp.MustCompile(`"mtime":[1-9][0-9]*`)
	countFields = regexp.MustCompile(`"(atime|ctime|btime|uid|gid)":[0-9]+`)
	homeField   = regexp.MustCompile(`"home":"([^"]*)"`)
)

// normalise replaces what differs between runs and machines: the image root
// becomes <root>, every modification time except the zero of a skipped row
// becomes "mtime":N, and the access, change and birth times and the owner ids
// become N. A caller checks the number of replacements.
func normalise(text, root string) (string, int) {
	text = strings.ReplaceAll(text, root, "<root>")
	replaced := 0
	text = timeFields.ReplaceAllStringFunc(text, func(string) string { replaced++; return `"mtime":N` })
	text = countFields.ReplaceAllStringFunc(text, func(field string) string {
		replaced++
		return field[:strings.Index(field, ":")+1] + "N"
	})
	return text, replaced
}

// normaliseRows returns the rows as normalise gives them, with the order the
// file system decides taken out: the rows of one volume come in the
// directory order of its tree, so each run of rows of one home is sorted;
// the order of the volumes is the code's and is kept. It also returns the
// number of fields normalise replaced.
func normaliseRows(t *testing.T, text, root string) (string, int) {
	t.Helper()
	text, replaced := normalise(text, root)
	var sorted, run []string
	runHome := ""
	for _, row := range strings.Split(text, "\n") {
		home := homeField.FindString(row)
		if home != runHome {
			slices.Sort(run)
			sorted, run, runHome = append(sorted, run...), nil, home
		}
		run = append(run, row)
	}
	slices.Sort(run)
	return strings.Join(append(sorted, run...), "\n"), replaced
}

// pinUmask sets the process umask to 022 until the test ends, so the modes
// of the files and directories the test writes, which its expected rows
// record, do not depend on the umask the tests were started with.
func pinUmask(test testing.TB) {
	previous := syscall.Umask(0o022)
	test.Cleanup(func() { syscall.Umask(previous) })
}

func TestCollectVolumesOverMixedImage(t *testing.T) {
	pinUmask(t)
	fixture := newSetup(t, false)
	fixture.collector.MaxSize = 1 << 20
	imageFixture(t, fixture)
	homes := []users.Home{{User: "alice", Path: filepath.Join(fixture.root, "home/alice")}}
	var notes []string
	got, err := Collect(context.Background(), fixture.collector, fixture.bases(), homes, &notes)
	if err != nil {
		t.Fatal(err)
	}
	want := Counts{Seen: true, Found: 6, Collected: 3, Desktop: true}
	if got != want {
		t.Errorf("counts = %+v, want %+v", got, want)
	}
	gotNotes, _ := normalise(strings.Join(notes, "\n"), fixture.root)
	if gotNotes != wantNotes {
		t.Errorf("notes:\n%s\nwant:\n%s", gotNotes, wantNotes)
	}
	gotLog, _ := normalise(strings.Join(fixture.log.lines, "\n"), fixture.root)
	if gotLog != wantLog {
		t.Errorf("log:\n%s\nwant:\n%s", gotLog, wantLog)
	}
	gotRows, replaced := normaliseRows(t, strings.TrimSuffix(fixture.rows.String(), "\n"), fixture.root)
	if gotRows != wantRows {
		t.Errorf("rows:\n%s\nwant:\n%s", gotRows, wantRows)
	}
	// Every row has six owner and time fields.
	if wantReplaced := 6 * 6; replaced != wantReplaced {
		t.Errorf("normalised %d fields, want %d", replaced, wantReplaced)
	}
}
