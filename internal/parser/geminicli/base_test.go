package geminicli

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func TestTheRuntimeDirectoryIsWhereTheCollectionHoldsIt(t *testing.T) {
	root := t.TempDir()
	home, staged := filepath.Join(root, "home"), filepath.Join(root, "staged", "tmp")
	write(t, filepath.Join(home, ".gemini", "projects.json"), `{"projects": {"/srv/proj": "proj"}}`)
	write(t, filepath.Join(staged, "proj", "logs.json"), `[]`)
	rel := ".gemini/tmp/proj/logs.json"
	tests := []struct {
		name     string
		artifact *input.Artifact
	}{
		{"a file of the home", &input.Artifact{Rel: rel,
			DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Home: &input.Home{DiskPath: home}}},
		{"a file the collection staged below a linked tmp/", &input.Artifact{Rel: rel,
			DiskPath: filepath.Join(staged, "proj", "logs.json"),
			Home:     &input.Home{DiskPath: home, StagedPaths: map[string]string{".gemini/tmp": staged}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			match := pathMatch{"base": ".gemini", "slug": "proj"}
			var problems parser.Problems
			if got := New().projectOf(test.artifact, match, &problems); got != "/srv/proj" || len(problems) != 0 {
				t.Errorf("got %q, %v; want the project projects.json names", got, problems)
			}
		})
	}
	orphan := &input.Artifact{Rel: rel, DiskPath: filepath.Join(home, filepath.FromSlash(rel))}
	if got := inBase(orphan, pathMatch{"base": ".gemini"})("projects.json"); got != "" {
		t.Errorf("an artifact with no home gave %q", got)
	}
}
