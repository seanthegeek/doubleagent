package parser

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
)

func TestBesideNamesAFileNextToTheArtifact(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home", "alice")
	artifact := &input.Artifact{DiskPath: filepath.Join(home, ".agent", "sessions", "one.jsonl"),
		Rel: ".agent/sessions/one.jsonl"}
	cases := []struct {
		name  string
		names []string
		want  string
	}{
		{"with no name it is the artifact's own directory", nil,
			filepath.Join(home, ".agent", "sessions")},
		{"one name is a file in that directory", []string{"state.db"},
			filepath.Join(home, ".agent", "sessions", "state.db")},
		{"several names go down from that directory", []string{"tool-results", "a.txt"},
			filepath.Join(home, ".agent", "sessions", "tool-results", "a.txt")},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			if got := Beside(artifact, each.names...); got != each.want {
				t.Errorf("Beside gave %q, want %q", got, each.want)
			}
		})
	}
}

func TestAboveStopsAtTheArtifactsHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home", "alice")
	artifact := &input.Artifact{DiskPath: filepath.Join(home, ".agent", "sessions", "one.jsonl"),
		Rel: ".agent/sessions/one.jsonl"}
	atHome := &input.Artifact{DiskPath: filepath.Join(home, "history.txt"), Rel: "history.txt"}
	cases := []struct {
		name     string
		artifact *input.Artifact
		levels   int
		names    []string
		want     string
		ok       bool
	}{
		{"level 0 is the artifact's own directory", artifact, 0, []string{"x"},
			filepath.Join(home, ".agent", "sessions", "x"), true},
		{"one level up is the directory above it", artifact, 1, []string{"config.json"},
			filepath.Join(home, ".agent", "config.json"), true},
		{"as many levels as the file lies below the home reach the home", artifact, 2, nil, home, true},
		{"one level more would leave the home", artifact, 3, nil, "", false},
		{"a file in the home itself cannot go up", atHome, 1, nil, "", false},
		{"a file in the home itself has the home as its directory", atHome, 0, nil, home, true},
		{"a negative level is refused", artifact, -1, nil, "", false},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			got, ok := Above(each.artifact, each.levels, each.names...)
			if got != each.want || ok != each.ok {
				t.Errorf("Above gave %q, %v; want %q, %v", got, ok, each.want, each.ok)
			}
		})
	}
}
