package pi

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestAParentPathFromTheEvidenceStaysInsideTheHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home", "alice")
	for _, rel := range []string{".pi/agent/sessions/p/parent.jsonl", "outside.jsonl"} {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := osfile.WriteFile(filepath.Join(root, "home", "escape.jsonl"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{Home: &input.Home{DiskPath: home, Original: "/home/alice"}}
	tests := []struct {
		name, original, want string
	}{
		{"a path below the home's original path is found", "/home/alice/.pi/agent/sessions/p/parent.jsonl",
			".pi/agent/sessions/p/parent.jsonl"},
		{"a path of another host is found from its .pi/ segment", `D:\Users\bob\.pi\agent\sessions\p\parent.jsonl`,
			".pi/agent/sessions/p/parent.jsonl"},
		{"empty components are dropped", "/home/alice//.pi/agent//sessions/p/parent.jsonl",
			".pi/agent/sessions/p/parent.jsonl"},
		{"a .. component climbing to a file of the home is refused", "/home/alice/.pi/../outside.jsonl", ""},
		{"a .. component climbing out of the home is refused", "/home/alice/../escape.jsonl", ""},
		{"a .. component after the .pi/ segment is refused", `D:\x\.pi\..\..\escape.jsonl`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := homeFile(artifact, test.original, ".pi/")
			want := ""
			if test.want != "" {
				want = filepath.Join(home, filepath.FromSlash(test.want))
			}
			if got != want || ok != (test.want != "") {
				t.Errorf("got %q, %v; want %q", got, ok, want)
			}
		})
	}
}
