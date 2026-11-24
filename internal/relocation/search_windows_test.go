package relocation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/shortname"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

func TestSweepRulesIgnoreCaseOnWindows(t *testing.T) {
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	// The homes and the project are real directories under the test's own
	// directory, so another spelling of one is the same entry wherever the
	// test runs; the home-root and output patterns compare text alone.
	base := syspath.FromNative(t.TempDir())
	for _, dir := range []string{"/Users/alice", "/Proj/.codex"} {
		if err := os.MkdirAll(syspath.ToNative(base+dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	run := &search{system: "C:", deps: Deps{
		Tables: tables, Homes: []users.Home{{User: "alice", Path: base + "/Users/alice"}},
		RunName: "host_run", Excluded: (&walk.Collector{Excludes: []string{"node_modules"}}).ExcludedByBytes,
		Claims: func(string, []catalog.Entry) []string { return []string{base + "/proj/.Codex"} },
	}}
	pruned := run.pruneTest(base+"/Proj", tables.ProjectCatalog, "project")
	cases := []struct {
		name      string
		got, want bool
	}{
		{"a home under another case of a home root is swept",
			run.underHomeRoot("c:/USERS/alice"), true},
		{"a home outside the home roots is not swept",
			run.underHomeRoot("C:/Windows/ServiceProfiles/x"), false},
		{"a project spelled in another case holds the home", run.holdsHome(base + "/users"), true},
		{"a project above the home in another case holds it",
			run.holdsHome(base + "/USERS/Alice"), true},
		{"a project beside the home holds none", run.holdsHome(base + "/Users/bob"), false},
		{"a catalog match is pruned in another case", pruned(base + "/PROJ/.codex"), true},
		{"the run's output file is pruned in another case", pruned("c:/out/HOST_RUN.log"), true},
		{"an exclusion is pruned in another case", pruned(base + "/proj/Node_Modules"), true},
		{"another file of the project is not pruned", pruned(base + "/Proj/notes.txt"), false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Errorf("got %v, want %v", test.got, test.want)
			}
		})
	}
}

func TestSweepPrunesTheRunsOutputBelowAShortName(t *testing.T) {
	short, long := shortname.Dir(t)
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(long, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The run's outputs are known by name, so a home spelled with a short
	// name prunes them as the long one does.
	for _, home := range []string{syspath.FromNative(long), syspath.FromNative(short)} {
		run := &search{system: "C:", deps: Deps{Tables: tables, RunName: "r",
			Homes:  []users.Home{{User: "u", Path: home}},
			Claims: func(string, []catalog.Entry) []string { return nil }}}
		// The sweep prunes a directory with what is under it, so the staging
		// directory itself is the path tested.
		pruned := run.pruneTest(home, tables.Catalog, "home")
		for _, path := range []string{home + "/out/.stage-r", home + "/out/R.log"} {
			if !pruned(path) {
				t.Errorf("%s is not pruned", path)
			}
		}
	}
}
