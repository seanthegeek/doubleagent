//go:build unix

package users

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEnumerateLive(t *testing.T) {
	root := tree{dirs: []string{"srv/getent", "srv/dscl", "srv/file", "home/alice"}}.build(t)
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/passwd"), []byte("file:x:1:1::/srv/file:/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	getent := func(name string, args ...string) ([]byte, error) {
		switch name {
		case "getent":
			return []byte("viagetent:x:1:1::/srv/getent:/bin/sh\n"), nil
		case "dscl":
			return []byte("viadscl  " + root + "/srv/dscl\nlonely\n"), nil
		}
		return nil, errors.New("unexpected " + name)
	}
	failing := func(string, ...string) ([]byte, error) { return nil, errors.New("not found") }
	cases := []struct {
		name string
		run  Runner
		want []Home
	}{
		{"getent replaces the passwd file, dscl adds, globs follow", getent, []Home{
			{"viagetent", root + "/srv/getent"}, {"viadscl", root + "/srv/dscl"},
			{"alice", root + "/home/alice"}}},
		{"programs that fail fall back to the passwd file", failing, []Home{
			{"file", root + "/srv/file"}, {"alice", root + "/home/alice"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// The fixture root stands in for a live run's empty root: getent's
			// homes are prefixed with it, dscl's are not.
			if got := Enumerate(true, root, "", testCase.run); !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Enumerate = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestFindReportsTheUsersFoundAndTheUnmatchedNames(t *testing.T) {
	root := tree{dirs: []string{"home/alice", "home/bob", "Users/Shared"}}.build(t)
	cases := []struct {
		name, filter string
		unmatched    []string
	}{
		{"a name in another case is unmatched, as Unix names are case-sensitive", "ALICE,bob",
			[]string{"ALICE"}},
		{"names that exist are all matched", "alice,bob", nil},
		{"no list reports nothing unmatched", "", nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			report := Find(false, root, test.filter, nil)
			if !reflect.DeepEqual(report.Unmatched, test.unmatched) ||
				!reflect.DeepEqual(report.Found, []string{"alice", "bob"}) {
				t.Errorf("unmatched %q, found %q; want %q and alice, bob", report.Unmatched,
					report.Found, test.unmatched)
			}
		})
	}
}
