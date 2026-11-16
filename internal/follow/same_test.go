package follow

import (
	"path/filepath"
	"testing"
)

func TestSameEntryLooksAtNothingTheRootCheckRefuses(t *testing.T) {
	missing := filepath.ToSlash(t.TempDir()) + "/missing"
	refused := func(path string) (Refusal, bool) { return Refusal{Link: path, Kind: LeadsOut}, true }
	if !SameEntry(nil)(missing, missing+"x") {
		t.Error("two paths that do not exist are not one entry")
	}
	if SameEntry(refused)(missing, missing+"x") {
		t.Error("a path the root check refuses was looked at")
	}
}
