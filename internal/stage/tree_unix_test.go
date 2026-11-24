//go:build darwin || linux

package stage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

func TestTheRootRefusesAPathOutOfItWithTheTextTheTreeMaps(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, err = root.Lstat("../outside")
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) || pathErr.Err.Error() != errPathEscapes {
		t.Errorf("os.Root.Lstat of a path out of it = %v; want the reason %q", err, errPathEscapes)
	}
}

func TestALinkOutOfTheTreeIsRefusedAsOutsideTheStagingDirectory(t *testing.T) {
	cases := []struct {
		name  string
		stage func(stager Stager, source string) error
	}{
		{"a copy", func(stager Stager, source string) error {
			_, err := stager.Copy(source)
			return err
		}},
		{"a link", func(stager Stager, source string) error {
			return symlink(stager, source, "target", false)
		}},
		{"a parent directory", func(stager Stager, source string) error {
			return makeParent(stager, source+"/child")
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, root := newStager(t)
			outside := t.TempDir()
			source := root + "/home/u/.tool/f"
			mustDo(t, os.MkdirAll(filepath.Dir(source), 0o755))
			mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
			// A link the run recreated in the staging tree, leading out of it.
			_, dest, _ := stager.Dest(filepath.Dir(source))
			mustDo(t, os.MkdirAll(filepath.Dir(dest), 0o700))
			mustDo(t, os.Symlink(outside, dest))
			if err := test.stage(stager, source); !leavesTree(err) {
				t.Errorf("staging through the link = %v; want the reason %q", err, outsideStage{})
			}
			if entries, _ := os.ReadDir(outside); len(entries) != 0 {
				t.Errorf("staged %v through the link", entries)
			}
		})
	}
}

func TestAStagingDirectoryMovedAsideStillTakesEveryCopy(t *testing.T) {
	stager, root := newStager(t)
	victim := t.TempDir()
	moved := filepath.Dir(stager.Dir) + "/moved"
	mustDo(t, os.Rename(stager.Dir, moved))
	mustDo(t, os.Symlink(victim, stager.Dir))
	source := root + "/home/u/f"
	mustDo(t, os.MkdirAll(filepath.Dir(source), 0o755))
	mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
	copied, err := stager.Copy(source)
	if err != nil {
		t.Fatal(err)
	}
	mustDo(t, os.Symlink("f", root+"/home/u/l"))
	mustDo(t, symlink(stager, root+"/home/u/l", "f", false))
	if entries, _ := os.ReadDir(victim); len(entries) != 0 {
		t.Errorf("staged %v where the link at the staging directory's name leads", entries)
	}
	if data, err := os.ReadFile(moved + "/" + copied.ArchivePath); err != nil || string(data) != "x" {
		t.Errorf("the moved staging directory holds %q, %v; want the copy", data, err)
	}
	if _, err := os.Lstat(moved + "/fs/home/u/l"); err != nil {
		t.Errorf("the link is not in the moved staging directory: %v", err)
	}
}

func TestCreateTreeTakesOnlyTheEmptyDirectoryItMade(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, path string)
		want  bool
	}{
		{"the empty directory just made is taken", func(*testing.T, string) {}, true},
		{"a directory holding an entry is refused", func(t *testing.T, path string) {
			mustDo(t, os.WriteFile(path+"/planted", nil, 0o600))
		}, false},
		{"a link at the name to an empty directory is refused", func(t *testing.T, path string) {
			mustDo(t, os.Remove(path))
			mustDo(t, os.Symlink(t.TempDir(), path))
		}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := t.TempDir() + "/stage"
			mustDo(t, os.Mkdir(path, 0o700))
			test.setup(t, path)
			if got := mustTree(t, path).madeByThisRun(); got != test.want {
				t.Errorf("madeByThisRun = %v, want %v", got, test.want)
			}
		})
	}
	t.Run("a name already taken is refused and left", func(t *testing.T) {
		path := t.TempDir() + "/stage"
		mustDo(t, os.Symlink(t.TempDir(), path))
		if tree, err := CreateTree(path); err == nil {
			_ = tree.Close()
			t.Fatal("CreateTree took a name a link held")
		}
		if info, err := os.Lstat(path); err != nil || info.Mode()&fs.ModeSymlink == 0 {
			t.Errorf("the link at the name was not left: %v", err)
		}
	})
}

func TestDiscardLeavesANameThatNoLongerHoldsTheTree(t *testing.T) {
	parent := t.TempDir()
	tree, err := CreateTree(parent + "/stage")
	if err != nil {
		t.Fatal(err)
	}
	mustDo(t, tree.MkdirAll("fs/a"))
	victim := t.TempDir()
	mustDo(t, os.WriteFile(victim+"/keep", []byte("x"), 0o600))
	mustDo(t, os.Rename(parent+"/stage", parent+"/moved"))
	mustDo(t, os.Symlink(victim, parent+"/stage"))
	if err := tree.Discard(); err == nil {
		t.Error("Discard removed a name that no longer holds the tree")
	}
	if entries, _ := os.ReadDir(parent + "/moved"); len(entries) != 0 {
		t.Errorf("the moved tree still holds %v", entries)
	}
	if _, err := os.Lstat(victim + "/keep"); err != nil {
		t.Errorf("the directory the planted link leads to lost its file: %v", err)
	}
	if info, err := os.Lstat(parent + "/stage"); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("the planted link was not left: %v", err)
	}
}

func TestADirectoryIsTakenOnlyWithTheOwnerANewEntryGets(t *testing.T) {
	self := uint32(os.Geteuid())
	cases := []struct {
		name               string
		directory, created uint32
		want               bool
	}{
		{"on an ordinary volume the effective user's directory is taken", self, self, true},
		{"on an ordinary volume another account's directory is refused", self + 1, self, false},
		{"on a volume giving every entry its mount's owner that owner is taken",
			self + 1000, self + 1000, true},
		{"on a volume that squashes the owner to nobody that owner is taken", 65534, 65534, true},
		{"on such a volume a directory with another owner is refused", self + 1000, 65534, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := sameOwnerAsNew(test.directory, test.created); got != test.want {
				t.Errorf("sameOwnerAsNew(%d, %d) = %v, want %v", test.directory, test.created,
					got, test.want)
			}
		})
	}
}

func TestTheOwnerProbeLeavesNothingBehind(t *testing.T) {
	path := t.TempDir() + "/stage"
	mustDo(t, os.Mkdir(path, 0o700))
	if !mustTree(t, path).madeByThisRun() {
		t.Fatal("madeByThisRun refused the empty directory just made")
	}
	if entries, _ := os.ReadDir(path); len(entries) != 0 {
		t.Errorf("the owner probe left %v", entries)
	}
}

// stagedResult is what staging one path gave: the member, the hash and the
// error text, compared between a tree that holds its parent and one that
// does not.
type stagedResult struct {
	member, hash, err string
}

// stageInTurn stages each path under root in order, a link when it ends in
// "l" and a copy otherwise, and returns what each gave. With fresh set the
// tree releases the chain it holds before each call, as a tree without it
// would walk every name from the root. Before the path at index moveAt the
// staging directory is moved aside and a link to victim put at its name.
func stageInTurn(t *testing.T, stager Stager, root string, paths []string, fresh bool,
	moveAt int, victim string,
) []stagedResult {
	t.Helper()
	var results []stagedResult
	for i, path := range paths {
		if i == moveAt {
			mustDo(t, os.Rename(stager.Dir, stager.Dir+"-moved"))
			mustDo(t, os.Symlink(victim, stager.Dir))
		}
		if fresh {
			stager.Tree.releaseParent()
		}
		result := stagedResult{}
		err := makeParent(stager, path)
		if fresh {
			stager.Tree.releaseParent()
		}
		if err == nil && strings.HasSuffix(path, "l") {
			result.member, err = stager.Link(path, "target", false)
		} else if err == nil {
			var copied Copied
			copied, err = stager.Copy(path)
			result.member, result.hash = copied.ArchivePath, copied.SHA256
		}
		if err != nil {
			result.err = strings.NewReplacer(stager.Dir, "{STAGE}", root, "{ROOT}").Replace(err.Error())
		}
		results = append(results, result)
	}
	return results
}

func TestTheParentTheTreeHoldsChangesNothingStaged(t *testing.T) {
	// In turn: a sibling, back, a child, a cousin several levels away and
	// back, links out of the tree as the parent's last component, below a
	// held level, beside one and as a middle component, and an in-tree
	// directory link as a middle component, with the staging directory moved
	// aside and a link put at its name midway.
	paths := []string{"/home/u/a/one", "/home/u/b/two", "/home/u/a/three", "/home/u/a/l",
		"/home/u/b/l", "/home/u/a/one", "/home/u/a/c/d/e/five", "/home/v/w/x/y/six",
		"/home/u/a/c/d/e/seven", "/home/u/a/c/d/eight", "/home/u/out/x", "/home/u/a/out/y",
		"/home/u/a/c/d/e/nine", "/home/u/out/deep/z", "/home/u/in/c/ten", "/home/u/in/c/d/e/l",
		"/home/u/four"}
	var runs [2][]stagedResult
	for i, fresh := range []bool{true, false} {
		stager, root := newStager(t)
		outside := t.TempDir()
		for _, path := range paths {
			mustDo(t, os.MkdirAll(filepath.Dir(root+path), 0o755))
			if strings.HasSuffix(path, "l") {
				_ = os.Symlink("target", root+path)
			} else {
				mustDo(t, os.WriteFile(root+path, []byte(path), 0o644))
			}
		}
		// Recreated links leading out of the tree: one beside a parent the
		// tree will hold, one below it.
		mustDo(t, os.MkdirAll(stager.Dir+"/fs/home/u/a", 0o700))
		mustDo(t, os.Symlink(outside, stager.Dir+"/fs/home/u/out"))
		mustDo(t, os.Symlink(outside, stager.Dir+"/fs/home/u/a/out"))
		mustDo(t, os.Symlink("a", stager.Dir+"/fs/home/u/in"))
		absolute := make([]string, len(paths))
		for j, path := range paths {
			absolute[j] = root + path
		}
		// The directory the planted link leads to mirrors the staged
		// directories, so a level reached by name instead of through the
		// chain would stage there.
		victim := t.TempDir()
		for _, path := range paths {
			mustDo(t, os.MkdirAll(victim+"/fs"+filepath.Dir(path), 0o700))
		}
		runs[i] = stageInTurn(t, stager, root, absolute, fresh, 9, victim)
		if entries, _ := os.ReadDir(outside); len(entries) != 0 {
			t.Errorf("staged %v through a link out of the tree (fresh %v)", entries, fresh)
		}
		requireOnlyDirectories(t, victim)
	}
	for j := range paths {
		if runs[0][j] != runs[1][j] {
			t.Errorf("%s: walked from the root %+v, through the held parent %+v", paths[j],
				runs[0][j], runs[1][j])
		}
	}
	for _, j := range []int{10, 11, 13} {
		if !strings.Contains(runs[1][j].err, outsideStage{}.Error()) {
			t.Errorf("%s through a link out of the tree = %q; want %q", paths[j], runs[1][j].err,
				outsideStage{})
		}
	}
}

// requireOnlyDirectories fails when anything but a directory is under dir.
func requireOnlyDirectories(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			t.Errorf("staged %s where the link at the staging directory's name leads", path)
		}
		return err
	})
	mustDo(t, err)
}

func TestAMemberIsSpelledFromTheOpenDirectoryNotItsName(t *testing.T) {
	stager, root := newStager(t)
	parent := filepath.Dir(stager.Dir)
	// The directory moved aside, and a link at its name to its own parent:
	// a name resolved now would put the copy under moved/.
	mustDo(t, os.Rename(stager.Dir, parent+"/moved"))
	mustDo(t, os.Symlink(parent, stager.Dir))
	source := root + "/home/u/f"
	mustDo(t, os.MkdirAll(filepath.Dir(source), 0o755))
	mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
	copied, err := stager.Copy(source)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ArchivePath != "fs/home/u/f" {
		t.Errorf("ArchivePath = %q, want fs/home/u/f, the member the archive holds",
			copied.ArchivePath)
	}
	if data, err := os.ReadFile(parent + "/moved/fs/home/u/f"); err != nil || string(data) != "x" {
		t.Errorf("the moved staging directory holds %q, %v; want the copy", data, err)
	}
}

func TestADirectoryWhoseOwnerProbeCannotBeMadeIsRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root creates files in any directory")
	}
	path := t.TempDir() + "/stage"
	mustDo(t, os.Mkdir(path, 0o500))
	t.Cleanup(func() { _ = os.Chmod(path, 0o700) })
	if mustTree(t, path).madeByThisRun() {
		t.Error("madeByThisRun took a directory in which the owner probe could not be made")
	}
}

func TestAMemberIsSpelledFromTheNameAfterTheTreeIsMovedInsideANewDirectory(t *testing.T) {
	stager, root := newStager(t)
	parent := filepath.Dir(stager.Dir)
	// mv stage tmp; mkdir stage; mv tmp stage/x: the system's path for a new
	// copy then begins with the staging directory's old path.
	mustDo(t, os.Rename(stager.Dir, parent+"/tmp"))
	mustDo(t, os.Mkdir(stager.Dir, 0o700))
	mustDo(t, os.Rename(parent+"/tmp", stager.Dir+"/x"))
	source := root + "/home/u/f"
	mustDo(t, os.MkdirAll(filepath.Dir(source), 0o755))
	mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
	copied, err := stager.Copy(source)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ArchivePath != "fs/home/u/f" {
		t.Errorf("ArchivePath = %q, want fs/home/u/f, the member the archive holds",
			copied.ArchivePath)
	}
}

func TestARespelledComponentGetsTheVolumesSpelling(t *testing.T) {
	stager, root := newStager(t)
	mustDo(t, os.MkdirAll(stager.Dir+"/fs/home/User", 0o700))
	if _, err := os.Lstat(stager.Dir + "/fs/home/user"); err != nil {
		t.Skip("the volume of the test's temporary directory tells letter case apart")
	}
	stager.FoldsCase = true
	source := root + "/home/user/f"
	mustDo(t, os.MkdirAll(filepath.Dir(source), 0o755))
	mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
	copied, err := stager.Copy(source)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ArchivePath != "fs/home/User/f" {
		t.Errorf("ArchivePath = %q, want fs/home/User/f, as the volume spells it",
			copied.ArchivePath)
	}
}

func TestACandidateSpellingNamingAnotherFileIsRefused(t *testing.T) {
	stager, root := newStager(t)
	source := root + "/home/u/f"
	mustDo(t, os.MkdirAll(filepath.Dir(source), 0o755))
	mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
	copied, err := stager.Copy(source)
	mustDo(t, err)
	// Another file whose trailing components a reported path could end in.
	mustDo(t, os.MkdirAll(stager.Dir+"/fs/home/v", 0o700))
	mustDo(t, os.WriteFile(stager.Dir+"/fs/home/v/f", []byte("other"), 0o600))
	info, err := os.Lstat(stager.Dir + "/fs/home/u/f")
	mustDo(t, err)
	_, dest, _ := stager.Dest(source)
	staged := fsmeta.StatIdentity(info)
	if got := stager.member("/elsewhere/fs/home/v/f", dest, staged); got != copied.ArchivePath {
		t.Errorf("member = %q; want %q: the candidate names another file", got, copied.ArchivePath)
	}
	if got := stager.member("/elsewhere/fs/home/u/f", dest, staged); got != "fs/home/u/f" {
		t.Errorf("member = %q for the candidate naming the file itself; want fs/home/u/f", got)
	}
}

func TestDiscardReportsBothATakenNameAndContentsLeft(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes from a directory it may not write")
	}
	parent := t.TempDir()
	tree, err := CreateTree(parent + "/stage")
	mustDo(t, err)
	mustDo(t, tree.MkdirAll("fs/shut"))
	file, err := tree.CreateNew("fs/shut/f", 0o600)
	mustDo(t, err)
	mustDo(t, file.Close())
	mustDo(t, os.Chmod(parent+"/stage/fs/shut", 0o500))
	t.Cleanup(func() { _ = os.Chmod(parent+"/moved/fs/shut", 0o700) })
	mustDo(t, os.Rename(parent+"/stage", parent+"/moved"))
	mustDo(t, os.Mkdir(parent+"/stage", 0o700))
	err = tree.Discard()
	if err == nil || !strings.Contains(err.Error(), notThisTree{}.Error()) ||
		!errors.Is(err, fs.ErrPermission) {
		t.Errorf("Discard = %v; want both the name taken and the contents that could not be removed",
			err)
	}
	if _, statErr := os.Lstat(parent + "/stage"); statErr != nil {
		t.Errorf("the directory now at the name was not left: %v", statErr)
	}
}

func TestACandidateNamingTheCreatedFileIsTheMember(t *testing.T) {
	stager, root := newStager(t)
	source := root + "/home/u/f"
	mustDo(t, os.MkdirAll(filepath.Dir(source), 0o755))
	mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
	_, err := stager.Copy(source)
	mustDo(t, err)
	// A second name of the very file the copy created, as a respelling
	// by the volume would give: the walk is not consulted (FoldsCase is
	// false), so only the candidate can give this spelling.
	mustDo(t, os.MkdirAll(stager.Dir+"/fs/home/v", 0o700))
	mustDo(t, os.Link(stager.Dir+"/fs/home/u/f", stager.Dir+"/fs/home/v/f"))
	info, err := os.Lstat(stager.Dir + "/fs/home/u/f")
	mustDo(t, err)
	_, dest, _ := stager.Dest(source)
	if got := stager.member("/elsewhere/fs/home/v/f", dest, fsmeta.StatIdentity(info)); got !=
		"fs/home/v/f" {
		t.Errorf("member = %q; want fs/home/v/f, the candidate naming the created file", got)
	}
}
