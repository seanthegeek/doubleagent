package relocation

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/users"
)

func TestSweepRulesIgnoreCaseOnWindows(t *testing.T) {
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	run := &search{system: "C:", deps: Deps{
		Tables: tables, Homes: []users.Home{{User: "alice", Path: "C:/Users/alice"}},
		OutDir: "C:/Out", RunName: "host_run", Excludes: []string{"node_modules"},
		Claims: func(string, []catalog.Entry) []string { return []string{"c:/proj/.Codex"} },
	}}
	pruned := run.pruneTest("C:/Proj", tables.ProjectCatalog, "project")
	cases := []struct {
		name      string
		got, want bool
	}{
		{"a home under another case of a home root is swept",
			run.underHomeRoot("c:/USERS/alice"), true},
		{"a home outside the home roots is not swept",
			run.underHomeRoot("C:/Windows/ServiceProfiles/x"), false},
		{"a project spelled in another case holds the home", run.holdsHome("c:/users"), true},
		{"a project above the home in another case holds it", run.holdsHome("C:/USERS/Alice"), true},
		{"a project beside the home holds none", run.holdsHome("C:/Users/bob"), false},
		{"a catalog match is pruned in another case", pruned("C:/PROJ/.codex"), true},
		{"the run's output file is pruned in another case", pruned("c:/out/HOST_RUN.log"), true},
		{"an exclusion is pruned in another case", pruned("C:/proj/Node_Modules"), true},
		{"another file of the project is not pruned", pruned("C:/Proj/notes.txt"), false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Errorf("got %v, want %v", test.got, test.want)
			}
		})
	}
}
