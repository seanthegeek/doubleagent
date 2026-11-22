//go:build unix

package collect

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/users"
)

func TestAStagingDirectoryMovedAsideMidRunChangesNothingTheRunWrites(t *testing.T) {
	root := fixtureImage(t)
	plain := t.TempDir()
	if code := runCollection(context.Background(), testOptions(root, plain), testHost(t),
		io.Discard, io.Discard); code != exitOK {
		t.Fatalf("the run nobody interferes with exits %d", code)
	}
	attacked, victim := t.TempDir(), t.TempDir()
	moved := attacked + "/moved"
	var planted string
	host := testHost(t)
	// The users step runs after the staging directory is made and before
	// anything is staged: the account that may rename entries in -o moves
	// it aside there and puts a link to its own directory at its name.
	host.findUsers = func(ctx context.Context, live bool, root, filter string, runner users.Runner,
		check glob.Check, opener *fsmeta.Opener, respell func(string) string) users.Report {
		planted = moveStageAside(t, attacked, moved, victim)
		return users.Find(ctx, live, root, filter, runner, check, opener, respell)
	}
	if code := runCollection(context.Background(), testOptions(root, attacked), host,
		io.Discard, io.Discard); code != exitOK {
		t.Fatalf("the run whose staging directory was moved exits %d", code)
	}
	requireSameMembers(t, archiveMembers(t, plain), archiveMembers(t, attacked))
	if names := outputNames(t, victim); len(names) != 0 {
		t.Errorf("the run wrote %v into the directory the planted link leads to", names)
	}
	if target, err := os.Readlink(planted); err != nil || target != victim {
		t.Errorf("the planted link reads %q, %v; want it left leading to %s", target, err, victim)
	}
	if names := outputNames(t, moved); len(names) != 0 {
		t.Errorf("the moved staging directory still holds %v", names)
	}
	if log := runOutput(t, attacked, ".log"); !strings.Contains(log,
		"staging directory not removed: ") {
		t.Errorf("the log does not say the staging directory's name was left:\n%s", log)
	}
}

// moveStageAside renames the one staging directory in outDir to moved,
// puts a link to victim at its name and returns the link's path.
func moveStageAside(t *testing.T, outDir, moved, victim string) string {
	t.Helper()
	for _, name := range outputNames(t, outDir) {
		if strings.HasPrefix(name, ".stage-") {
			if err := os.Rename(outDir+"/"+name, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(victim, outDir+"/"+name); err != nil {
				t.Fatal(err)
			}
			return outDir + "/" + name
		}
	}
	t.Fatalf("no staging directory in %s", outDir)
	return ""
}

// requireSameMembers fails unless the two archives hold the same members,
// and each member under fs/ the same bytes; the run's own files differ in
// the output directory they name.
func requireSameMembers(t *testing.T, want, got map[string]string) {
	t.Helper()
	for name, hash := range want {
		other, found := got[name]
		if !found || strings.HasPrefix(name, "./fs/") && other != hash {
			t.Errorf("member %s is missing or holds other bytes (%q, want %q)", name, other, hash)
		}
	}
	if len(got) != len(want) {
		t.Errorf("the archive holds %d members, want %d: %v", len(got), len(want), got)
	}
}

func TestTheWriteProbeRemovesNothingThroughALinkPutAtItsDirectorysName(t *testing.T) {
	parent, victim := t.TempDir(), t.TempDir()
	dir := parent + "/out"
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var probeName string
	writable := probeWriteThen(dir, func(probe string) {
		probeName = probe
		// The account that may rename entries in the parent moves the
		// directory aside and puts a link at its name to a directory
		// holding a file of the probe's name.
		if err := os.WriteFile(victim+"/"+probe, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(dir, parent+"/moved"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(victim, dir); err != nil {
			t.Fatal(err)
		}
	})
	if !writable {
		t.Fatal("probeWriteThen = false for a writable directory")
	}
	if data, err := os.ReadFile(victim + "/" + probeName); err != nil || string(data) != "keep" {
		t.Errorf("the file the link leads to reads %q, %v; want it left", data, err)
	}
	if names := outputNames(t, parent+"/moved"); len(names) != 0 {
		t.Errorf("the probe's own directory still holds %v", names)
	}
}
