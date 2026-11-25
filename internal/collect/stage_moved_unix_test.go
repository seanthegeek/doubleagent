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
		io.Discard, io.Discard); code != ExitOK {
		t.Fatalf("the run nobody interferes with exits %d", code)
	}
	want := archiveMembers(t, plain)
	for _, test := range stageMoves() {
		t.Run(test.name, func(t *testing.T) {
			outDir := t.TempDir()
			var moved, stage string
			var after func(t *testing.T)
			runWithStageMoved(t, root, outDir, func(path string) {
				stage = path
				moved, after = test.move(t, outDir, path)
			})
			requireSameMembers(t, want, archiveMembers(t, outDir))
			requireRowsNameMembers(t, outDir)
			after(t)
			if names := outputNames(t, moved); len(names) != 0 {
				t.Errorf("the moved staging directory still holds %v", names)
			}
			if log := runOutput(t, outDir, ".log"); !strings.Contains(log,
				"staging directory not removed: remove "+stage+": ") {
				t.Errorf("the log does not say the staging directory's name was left:\n%s", log)
			}
		})
	}
}

// stageMove is one way an account that may rename entries in -o moves the
// staging directory aside mid-run: move does it to the directory at stage
// in outDir and returns where the directory now is and the checks of what
// is left at the name and where any link leads.
type stageMove struct {
	name string
	move func(t *testing.T, outDir, stage string) (string, func(t *testing.T))
}

// stageMoves are the moves the end-to-end test makes: a link at the name
// to the other account's directory, a link to the output directory itself,
// and the directory moved inside a new directory made at its name.
func stageMoves() []stageMove {
	linkTo := func(target func(t *testing.T, outDir string) string) func(*testing.T, string,
		string) (string, func(*testing.T)) {
		return func(t *testing.T, outDir, stage string) (string, func(*testing.T)) {
			leads := target(t, outDir)
			before := outputNames(t, leads)
			mustRename(t, stage, outDir+"/moved")
			if err := os.Symlink(leads, stage); err != nil {
				t.Fatal(err)
			}
			return outDir + "/moved", func(t *testing.T) {
				requireNothingWrittenThrough(t, leads, outDir, before)
				if link, err := os.Readlink(stage); err != nil || link != leads {
					t.Errorf("the planted link reads %q, %v; want it left leading to %s", link,
						err, leads)
				}
			}
		}
	}
	return []stageMove{
		{"a link to the other account's own directory", linkTo(func(t *testing.T, _ string) string {
			return t.TempDir()
		})},
		{"a link to the output directory itself", linkTo(func(_ *testing.T, outDir string) string {
			return outDir
		})},
		{"the directory moved inside a new directory at its name",
			func(t *testing.T, outDir, stage string) (string, func(*testing.T)) {
				mustRename(t, stage, outDir+"/tmp")
				if err := os.Mkdir(stage, 0o700); err != nil {
					t.Fatal(err)
				}
				mustRename(t, outDir+"/tmp", stage+"/x")
				return stage + "/x", func(t *testing.T) {
					if names := outputNames(t, stage); len(names) != 1 || names[0] != "x" {
						t.Errorf("the new directory at the name holds %v, want only x", names)
					}
				}
			}},
	}
}

// mustRename renames source to target or fails the test.
func mustRename(t *testing.T, source, target string) {
	t.Helper()
	if err := os.Rename(source, target); err != nil {
		t.Fatal(err)
	}
}

// runWithStageMoved runs a collection of root into outDir, calling move
// with the staging directory's path during the users step, after the
// directory is made and before anything is staged.
func runWithStageMoved(t *testing.T, root, outDir string, move func(stage string)) {
	t.Helper()
	host := testHost(t)
	host.findUsers = func(ctx context.Context, live bool, root, filter string, runner users.Runner,
		check glob.Check, opener *fsmeta.Opener, respell func(string) string) users.Report {
		for _, name := range outputNames(t, outDir) {
			if strings.HasPrefix(name, ".stage-") {
				move(outDir + "/" + name)
			}
		}
		return users.Find(ctx, live, root, filter, runner, check, opener, respell)
	}
	if code := runCollection(context.Background(), testOptions(root, outDir), host,
		io.Discard, io.Discard); code != ExitOK {
		t.Fatalf("the run whose staging directory was moved exits %d", code)
	}
}

// requireNothingWrittenThrough fails when target, where the planted link
// leads, holds a name it did not hold before the run, other than the
// run's own outputs and the moved directory when target is the output
// directory outDir.
func requireNothingWrittenThrough(t *testing.T, target, outDir string, before []string) {
	t.Helper()
	known := map[string]bool{"moved": true}
	for _, name := range before {
		known[name] = true
	}
	for _, name := range outputNames(t, target) {
		own := target == outDir && (strings.Contains(name, "_agent-artifacts") ||
			strings.HasPrefix(name, ".stage-"))
		if !known[name] && !own {
			t.Errorf("the run wrote %s where the planted link leads", name)
		}
	}
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
