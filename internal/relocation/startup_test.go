package relocation

import (
	"github.com/seanthegeek/doubleagent/internal/follow"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

func TestStartupFilesListNothingBehindALinkOutOfTheRoot(t *testing.T) {
	cases := []struct {
		name         string
		inside, live bool
		wantFiles    bool
		wantRefused  bool
	}{
		{"a link out of the image root is not listed", false, false, false, true},
		{"a link inside the home is listed through", true, false, true, false},
		{"a live run lists through it and drops what leads out of the base", false, true,
			false, false},
	}
	table := []catalog.StartupFile{{Scope: "home", Glob: ".bashrc.d/*"}}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
			home := root + "/home/alice"
			target := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
			if testCase.inside {
				target = home + "/dotfiles"
			}
			for _, dir := range []string{home, target} {
				if err := os.MkdirAll(syspath.ToNative(dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			file := filepath.Join(syspath.ToNative(target), "analyst.sh")
			if err := os.WriteFile(file, []byte("export CODEX_HOME=/x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			linkOut(t, syspath.ToNative(target), syspath.ToNative(home+"/.bashrc.d"))
			check := walk.NewRootCheck(root).Expansion()
			if testCase.live {
				check = nil
			}
			files, refused := ListStartupFiles(home, "home", table, check, nil, stage.RunOutput{})
			if (len(files) > 0) != testCase.wantFiles {
				t.Errorf("files %q, want some: %v", files, testCase.wantFiles)
			}
			var want []follow.Refusal
			if testCase.wantRefused {
				want = []follow.Refusal{{Link: home + "/.bashrc.d", Kind: follow.LeadsOut}}
			}
			if !reflect.DeepEqual(refused, want) {
				t.Errorf("refused %+v, want %+v", refused, want)
			}
		})
	}
}

func TestStartupFilesListNoneOfTheRunsOwnOutputs(t *testing.T) {
	home := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	for _, name := range []string{"x", "r.log", "r.manifest.jsonl", "rx"} {
		path := syspath.ToNative(home + "/.bashrc.d/" + name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("export CODEX_HOME=/x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	table := []catalog.StartupFile{{Scope: "home", Glob: ".bashrc.d/*"}}
	files, _ := ListStartupFiles(home, "home", table, nil, nil, stage.RunOutput{RunName: "r"})
	if want := []string{home + "/.bashrc.d/rx", home + "/.bashrc.d/x"}; !reflect.DeepEqual(files,
		want) {
		t.Errorf("files %q, want only %q", files, want)
	}
}
