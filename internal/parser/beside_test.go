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
			if got, ok := Beside(artifact, each.names...); got != each.want || !ok {
				t.Errorf("Beside gave %q, %v; want %q", got, ok, each.want)
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

// homeArtifact returns an artifact of a home at root/home whose path below
// the home is rel and whose disk path is diskPath below root.
func homeArtifact(root, rel, diskPath string, staged map[string]string) *input.Artifact {
	home := &input.Home{DiskPath: filepath.Join(root, "home"), StagedPaths: staged}
	return &input.Artifact{Rel: rel, DiskPath: filepath.Join(root, filepath.FromSlash(diskPath)), Home: home}
}

func TestNeighboursAreFoundBelowTheHomeWhateverTheDiskPathSays(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	shallow := homeArtifact(root, "a/b/c/d/x.json", "x.json", nil)
	staged := homeArtifact(root, ".pi/agent/s.jsonl", "home/dotfiles/pi/agent/s.jsonl",
		map[string]string{".pi": filepath.Join(home, "dotfiles", "pi"),
			".pi/agent": filepath.Join(home, "dotfiles", "pi", "agent")})
	cases := []struct {
		name     string
		artifact *input.Artifact
		levels   int
		names    []string
		want     string
		ok       bool
	}{
		{"a shallow disk path climbs the home's own directories", shallow, 4, []string{"_index.json"},
			filepath.Join(home, "_index.json"), true},
		{"a shallow disk path cannot climb past the home", shallow, 5, nil, "", false},
		{"a name that climbs is refused", shallow, 0, []string{"..", "..", "etc"}, "", false},
		{"a name holding a slash is refused", shallow, 0, []string{"../../etc"}, "", false},
		{"an empty name is refused", shallow, 0, []string{""}, "", false},
		{"a staged file's neighbour is where the home's layout holds it", staged, 0, []string{"x"},
			filepath.Join(home, "dotfiles", "pi", "agent", "x"), true},
		{"a staged file's parent is the staged parent", staged, 1, []string{"settings.json"},
			filepath.Join(home, "dotfiles", "pi", "settings.json"), true},
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

func TestUnderRefusesANameThatIsNotOneComponent(t *testing.T) {
	directory := t.TempDir()
	for _, names := range [][]string{{".."}, {"a/b"}, {`a\b`}, {"a", ""}, {"a\x00"}} {
		if path, ok := Under(directory, names...); ok {
			t.Errorf("Under(%q) gave %q", names, path)
		}
	}
	if path, ok := Under(directory, "a", "b"); !ok || path != filepath.Join(directory, "a", "b") {
		t.Errorf("Under gave %q, %v", path, ok)
	}
}

func TestADirectorysChildIsOneNameBelowIt(t *testing.T) {
	root := t.TempDir()
	directory, ok := DirectoryAt(root)
	if !ok {
		t.Fatal("a real directory was refused")
	}
	cases := []struct {
		name      string
		directory Directory
		child     string
		want      string
		ok        bool
	}{
		{"a plain name is joined below the directory", directory, "a.json", filepath.Join(root, "a.json"), true},
		{"a dot file is a plain name", directory, ".workspace", filepath.Join(root, ".workspace"), true},
		{"a name that climbs is refused", directory, "..", "", false},
		{"the directory itself is refused", directory, ".", "", false},
		{"a name holding a slash is refused", directory, "../etc", "", false},
		{"a name holding a backslash is refused", directory, `..\etc`, "", false},
		{"an empty name is refused", directory, "", "", false},
		{"a name holding a NUL byte is refused", directory, "a\x00b", "", false},
		{"the zero directory has no child", Directory{}, "a.json", "", false},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			got, ok := each.directory.Child(each.child)
			if got != each.want || ok != each.ok {
				t.Errorf("Child gave %q, %v; want %q, %v", got, ok, each.want, each.ok)
			}
		})
	}
}

func TestSidecarNamesOnlyADatabasesOwnFiles(t *testing.T) {
	database := filepath.Join("home", "state.vscdb")
	for _, test := range []struct {
		suffix, want string
		ok           bool
	}{
		{"-wal", database + "-wal", true},
		{"-shm", database + "-shm", true},
		{"-journal", database + "-journal", true},
		{"/../x", "", false},
		{"-wal/x", "", false},
		{"", "", false},
	} {
		if got, ok := Sidecar(database, test.suffix); got != test.want || ok != test.ok {
			t.Errorf("Sidecar(%q) = %q, %v; want %q, %v", test.suffix, got, ok, test.want, test.ok)
		}
	}
	if _, ok := Sidecar("", "-wal"); ok {
		t.Error("Sidecar of no database gave a path")
	}
}
