package follow

import (
	"errors"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestFollowingFunctionsRefuseWhatTheCheckRefuses(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	if err := os.MkdirAll(filepath.Join(syspath.ToNative(root), "in", "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	fenced := root + "/in"
	refusing := func(path string) (Refusal, bool) {
		return Refusal{Link: fenced, Kind: LeadsOut}, path == fenced+"/dir"
	}
	calls := []struct {
		name string
		run  func(check Check) error
	}{
		{"Stat", func(check Check) error { _, err := Stat(check, fenced+"/dir"); return err }},
		{"IsDirectory", func(check Check) error {
			_, err := IsDirectory(check, fenced+"/dir")
			return err
		}},
		{"IsRegularFile", func(check Check) error {
			_, err := IsRegularFile(check, fenced+"/dir")
			return err
		}},
		{"ReadFile", func(check Check) error { _, err := ReadFile(check, fenced+"/dir"); return err }},
		{"DirReadError", func(check Check) error {
			_, _, err := DirReadError(check, nil, fenced+"/dir")
			return err
		}},
		{"PhysicalDir", func(check Check) error {
			_, err := PhysicalDir(check, fenced+"/dir")
			return err
		}},
		{"CanRead", func(check Check) error { _, err := CanRead(check, nil, fenced+"/dir"); return err }},
		{"CanEnter", func(check Check) error { _, err := CanEnter(check, fenced+"/dir"); return err }},
	}
	for _, call := range calls {
		t.Run(call.name+" refuses a path the check refuses and follows nothing", func(t *testing.T) {
			err := call.run(refusing)
			if refusal, out := OutOfRoot(err); !out || refusal.Link != fenced {
				t.Errorf("error %v, want an *OutOfRootError at %s", err, fenced)
			}
		})
		t.Run(call.name+" with no check follows as the system does", func(t *testing.T) {
			if _, out := OutOfRoot(call.run(nil)); out {
				t.Error("a live call was refused")
			}
		})
	}
}

func TestDirReadErrorReportsARefusalAsFailed(t *testing.T) {
	refusing := func(path string) (Refusal, bool) { return Refusal{Link: path, Kind: LeadsOut}, true }
	reason, failed, err := DirReadError(refusing, nil, "/img/home/link")
	if !failed || err == nil || reason != err.Error() || reason == "" {
		t.Errorf("DirReadError = %q, %v, %v; want failed, with the refusal as the reason",
			reason, failed, err)
	}
}

func TestIsDirectoryAnswersWithoutACheckRefusal(t *testing.T) {
	directory := syspath.FromNative(t.TempDir())
	cases := []struct {
		name, path string
		want       bool
	}{
		{"a directory is one", directory, true},
		{"a missing path is none, with no error", directory + "/missing", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := IsDirectory(nil, testCase.path)
			if got != testCase.want || err != nil {
				t.Errorf("IsDirectory = %v, %v; want %v, nil", got, err, testCase.want)
			}
		})
	}
	var refusal *OutOfRootError
	if errors.As(errors.New("other"), &refusal) {
		t.Error("an unrelated error read as a refusal")
	}
}

func TestPlainEntriesAndLinksAreNoOtherReparsePoint(t *testing.T) {
	directory := syspath.FromNative(t.TempDir())
	paths := []string{directory, directory + "/missing"}
	link := directory + "/link"
	err := os.Symlink(syspath.ToNative(directory), syspath.ToNative(link))
	if err == nil {
		paths = append(paths, link)
	} else {
		t.Logf("no symlink case, one cannot be made here: %v", err)
	}
	for _, path := range paths {
		if meta, err := fsmeta.Lstat(path); err == nil && meta.IsOtherReparsePoint() {
			t.Errorf("%s reads as a reparse point other than a symlink or junction", path)
		}
	}
}

func TestReasonWordsEachKindOfRefusal(t *testing.T) {
	const leadsOut = ": the link leads out"
	cases := []struct {
		kind Kind
		want string
	}{
		{LeadsOut, leadsOut},
		{Unverified, " cannot be verified to lie inside the image root; nothing behind it was looked at"},
		{OtherReparse, " is a reparse point other than a symlink or junction; " +
			"nothing behind it was looked at"},
		{Climbs, " climbs above the image root with ..; nothing behind it was looked at"},
		{Outside, " is outside the image root; nothing behind it was looked at"},
	}
	for _, testCase := range cases {
		if got := Reason(Refusal{Link: "/img/l", Kind: testCase.kind}, leadsOut); got != testCase.want {
			t.Errorf("Reason(kind %d) = %q, want %q", testCase.kind, got, testCase.want)
		}
	}
}

func TestCanReadTakesOnlyADirectory(t *testing.T) {
	root := t.TempDir()
	file, empty := filepath.Join(root, "file"), filepath.Join(root, "empty")
	if err := os.WriteFile(file, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Run("a readable file is not a directory that can be read", func(t *testing.T) {
		if readable, err := CanRead(nil, nil, syspath.FromNative(file)); readable || err != nil {
			t.Errorf("CanRead = %v, %v; want false, nil", readable, err)
		}
	})
	t.Run("an empty directory can be read and entered", func(t *testing.T) {
		readable, readErr := CanRead(nil, nil, syspath.FromNative(empty))
		enterable, enterErr := CanEnter(nil, syspath.FromNative(empty))
		if !readable || !enterable || readErr != nil || enterErr != nil {
			t.Errorf("CanRead = %v, %v, CanEnter = %v, %v; want true, nil twice", readable,
				readErr, enterable, enterErr)
		}
	})
}
