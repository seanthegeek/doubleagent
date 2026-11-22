//go:build unix

package analyze

import (
	"os"
	"testing"
)

func TestTheWriteProbeRemovesNothingThroughALinkPutAtItsDirectorysName(t *testing.T) {
	parent, victim := t.TempDir(), t.TempDir()
	directory := parent + "/out"
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	var probeName string
	writable := probeWriteThen(directory, func(probe string) {
		probeName = probe
		// The account that may rename entries in the parent moves the
		// directory aside and puts a link at its name to a directory
		// holding a file of the probe's name.
		if err := os.WriteFile(victim+"/"+probe, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(directory, parent+"/moved"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(victim, directory); err != nil {
			t.Fatal(err)
		}
	})
	if !writable {
		t.Fatal("probeWriteThen = false for a writable directory")
	}
	if data, err := os.ReadFile(victim + "/" + probeName); err != nil || string(data) != "keep" {
		t.Errorf("the file the link leads to reads %q, %v; want it left", data, err)
	}
	if entries, _ := os.ReadDir(parent + "/moved"); len(entries) != 0 {
		t.Errorf("the probe's own directory still holds %v", entries)
	}
}

func TestTheWriteProbeTakesTheDirectoryAsTheAnalystGivesIt(t *testing.T) {
	cases := []struct {
		name string
		// dir returns the directory as given, and the directory to list.
		dir func(t *testing.T) (string, string)
	}{
		{"a directory given as it is", func(t *testing.T) (string, string) {
			dir := t.TempDir()
			return dir, dir
		}},
		{"one given with a trailing slash", func(t *testing.T) (string, string) {
			dir := t.TempDir()
			return dir + "/", dir
		}},
		{"one given relative to the working directory", func(t *testing.T) (string, string) {
			parent := t.TempDir()
			if err := os.Mkdir(parent+"/out", 0o700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(parent)
			return "out", parent + "/out"
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			given, listed := test.dir(t)
			if !probeWrite(given) {
				t.Errorf("probeWrite(%s) = false for a writable directory", given)
			}
			if entries, err := os.ReadDir(listed); err != nil || len(entries) != 0 {
				t.Errorf("the probe left %v (%v)", entries, err)
			}
		})
	}
}
