//go:build unix

package users

import (
	"context"
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
	getent := func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch name {
		case "getent":
			return []byte("viagetent:x:1:1::/srv/getent:/bin/sh\n"), nil
		case "dscl":
			return []byte("viadscl  " + root + "/srv/dscl\nlonely\n"), nil
		}
		return nil, errors.New("unexpected " + name)
	}
	failing := func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("not found")
	}
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
			got := Enumerate(context.Background(), true, root, "", testCase.run, nil, nil)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Enumerate = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestFindReportsTheUsersFoundAndTheUnmatchedNames(t *testing.T) {
	root := tree{dirs: []string{"home/alice", "etc"}}.build(t)
	passwd := "nobody:x:65534:65534::/nonexistent:/bin/false\n" +
		"www:x:33:33::/var/www:/bin/false\n" + "alice:x:1000:1000::/home/alice:/bin/sh\n"
	if err := os.WriteFile(filepath.Join(root, "etc", "passwd"), []byte(passwd), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, filter string
		unmatched    []string
	}{
		{"users without a home directory are unmatched", "nobody,www,alice", []string{"nobody", "www"}},
		{"a name in another case is unmatched, as Unix names are case-sensitive", "ALICE",
			[]string{"ALICE"}},
		{"repeats are reported once, in the order typed", "www,alice,www", []string{"www"}},
		{"no list reports nothing unmatched", "", nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			report := Find(context.Background(), false, root, test.filter, nil, nil, nil)
			if !reflect.DeepEqual(report.Unmatched, test.unmatched) ||
				!reflect.DeepEqual(report.Found, []string{"alice"}) {
				t.Errorf("unmatched %q, found %q; want %q and alice only", report.Unmatched,
					report.Found, test.unmatched)
			}
		})
	}
}

func TestFindRunsNoProgramOnceCancelled(t *testing.T) {
	root := tree{dirs: []string{"home/alice"}}.build(t)
	cases := []struct {
		name      string
		cancelled bool
		want      []string
	}{
		{"a live run asks getent and dscl", false, []string{"getent", "dscl"}},
		{"a cancelled run asks neither", true, nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if testCase.cancelled {
				cancel()
			}
			var calls []string
			recording := func(_ context.Context, name string, _ ...string) ([]byte, error) {
				calls = append(calls, name)
				return nil, errors.New(name + ": not installed")
			}
			Find(ctx, true, root, "", recording, nil, nil)
			if !reflect.DeepEqual(calls, testCase.want) {
				t.Errorf("calls = %q, want %q", calls, testCase.want)
			}
		})
	}
}
