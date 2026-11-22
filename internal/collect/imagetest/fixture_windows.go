package imagetest

import (
	"path/filepath"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// removeTree deletes path. A deny rule a test added is taken away by its own
// cleanup, which runs before this one.
func removeTree(path string) {
	_ = osfile.RemoveAll(path)
}

// symlinkOrNote creates a symlink at path to target and reports whether it
// could; where the account may not create symlinks it logs a note instead
// of failing, so the test runs with the rest of its image.
func (f *fixture) symlinkOrNote(target, path string) bool {
	f.test.Helper()
	f.dir(filepath.Dir(path))
	// As os.Symlink does, a target that is a directory, found relative to the
	// link's directory, makes a directory link.
	located := target
	if !filepath.IsAbs(target) {
		located = filepath.Join(filepath.Dir(path), target)
	}
	kind := KindLink
	if info, err := osfile.Stat(located); err == nil && info.IsDir() {
		kind = KindDirLink
	}
	skipped, err := makeLink(path, target, kind)
	if err != nil {
		f.test.Fatal(err)
	}
	if skipped != "" {
		f.test.Logf("note: the test runs without the link %s: %s", path, skipped)
		return false
	}
	f.noteNew(path)
	return true
}

// junction makes link a junction to the directory target and removes it at
// the end.
func (f *fixture) junction(target, link string) {
	f.test.Helper()
	f.dir(filepath.Dir(link))
	f.noteNew(link)
	if err := makeJunction(target, link); err != nil {
		f.test.Fatal(err)
	}
}

// deny adds a rule denying the current user the icacls rights (such as
// "RD" or "WD,AD") on path, and puts the access list back at the end.
func (f *fixture) deny(path, rights string) {
	f.test.Helper()
	undo, err := denyAccess(path, rights)
	if err != nil {
		f.test.Fatalf("deny %s on %s: %v", rights, path, err)
	}
	f.test.Cleanup(func() {
		if err := undo(); err != nil {
			f.test.Errorf("restoring the access list of %s: %v", path, err)
		}
	})
}

// denyListing denies listing the directory at path, and skips the test
// where the directory can still be listed: an enabled SeBackupPrivilege lets
// the listing through the rule.
func (f *fixture) denyListing(path string) {
	f.test.Helper()
	f.deny(path, "RD")
	if _, err := osfile.ReadDir(path); err == nil {
		f.test.Skip("the deny rule does not stop this account listing the directory " +
			"(SeBackupPrivilege is enabled)")
	}
}
