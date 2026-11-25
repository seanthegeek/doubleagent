//go:build unix

package users

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
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
			got := Find(context.Background(), true, root, "", testCase.run, nil, nil, nil).Homes
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Find = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestALiveRunReportsTheUsersFoundAndTheUnmatchedNames runs live over a
// fixture root, getent and dscl failing, so the passwd file is read: a live
// run compares -u names byte for byte.
func TestALiveRunReportsTheUsersFoundAndTheUnmatchedNames(t *testing.T) {
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
			failing := func(context.Context, string, ...string) ([]byte, error) {
				return nil, errors.New("not installed")
			}
			report := Find(context.Background(), true, root, test.filter, failing, nil, nil, nil)
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
			Find(ctx, true, root, "", recording, nil, nil, nil)
			if !reflect.DeepEqual(calls, testCase.want) {
				t.Errorf("calls = %q, want %q", calls, testCase.want)
			}
		})
	}
}

// TestFindReportsAHomeItCannotExamine has passwd homes below a directory
// that can be listed but not entered and below one that can be neither, a
// missing home and a home that is a file: the first two are skipped, Denied
// only where the parent lists the home, and the last two are dropped.
func TestFindReportsAHomeItCannotExamine(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root passes every permission check")
	}
	root := tree{dirs: []string{"etc", "listed/alice", "closed/bob"}, files: []string{"srv/file"}}.build(t)
	passwd := "alice:x:1:1::/listed/alice:/bin/sh\nbob:x:2:2::/closed/bob:/bin/sh\n" +
		"ghost:x:3:3::/srv/ghost:/bin/sh\nfile:x:4:4::/srv/file:/bin/sh\n" +
		"below:x:5:5::/srv/file/sub:/bin/sh\n"
	if err := os.WriteFile(filepath.Join(root, "etc", "passwd"), []byte(passwd), 0o644); err != nil {
		t.Fatal(err)
	}
	for dir, mode := range map[string]os.FileMode{"listed": 0o444, "closed": 0} {
		path := filepath.Join(root, dir)
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
	}
	report := Find(context.Background(), false, root, "", nil, nil, nil, nil)
	if slices.Contains(report.Found, "below") {
		t.Errorf("found %q: a home below a regular file does not exist", report.Found)
	}
	var got []string
	for _, skipped := range report.Skipped {
		got = append(got, fmt.Sprintf("%s denied=%t", skipped.Path[len(root):], skipped.Denied))
		if !strings.Contains(skipped.Reason, "permission denied") {
			t.Errorf("reason %q lacks permission denied", skipped.Reason)
		}
	}
	want := []string{"/listed/alice denied=true", "/closed/bob denied=false"}
	if !reflect.DeepEqual(got, want) || len(report.Homes) != 0 ||
		!reflect.DeepEqual(report.Found, []string{"alice", "bob"}) {
		t.Errorf("skipped %q, homes %v, found %q; want %q, no home, alice and bob", got,
			report.Homes, report.Found, want)
	}
}

// A skipped home names its passwd user, not the last segment of its path.
func TestASkippedHomeNamesItsPasswdUser(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root passes every permission check")
	}
	root := tree{dirs: []string{"etc", "staff/j.doe"}}.build(t)
	passwd := "jdoe:x:1001:1001::/staff/j.doe:/bin/sh\n"
	if err := os.WriteFile(filepath.Join(root, "etc", "passwd"), []byte(passwd), 0o644); err != nil {
		t.Fatal(err)
	}
	staff := filepath.Join(root, "staff")
	if err := os.Chmod(staff, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(staff, 0o755) })
	report := Find(context.Background(), false, root, "", nil, nil, nil, nil)
	if len(report.Skipped) != 1 || report.Skipped[0].User != "jdoe" ||
		!reflect.DeepEqual(report.Found, []string{"jdoe"}) {
		t.Errorf("skipped %+v, found %q; want one skipped home of jdoe", report.Skipped,
			report.Found)
	}
}

// TestANamedPipeAtThePasswdFileIsNotRead has a named pipe, then a link to
// one, at the image's etc/passwd: opening it would wait for a writer for
// ever, so it gives no passwd homes and the home parents are still searched.
func TestANamedPipeAtThePasswdFileIsNotRead(t *testing.T) {
	for _, linked := range []bool{false, true} {
		root := tree{dirs: []string{"etc", "home/alice"}}.build(t)
		pipe := filepath.Join(root, "etc", "passwd")
		if linked {
			pipe = filepath.Join(root, "etc", "pipe")
			if err := os.Symlink("pipe", filepath.Join(root, "etc", "passwd")); err != nil {
				t.Fatal(err)
			}
		}
		if err := syscall.Mkfifo(pipe, 0o644); err != nil {
			t.Skipf("cannot make a named pipe here: %v", err)
		}
		done := make(chan Report, 1)
		go func() { done <- Find(context.Background(), false, root, "", nil, nil, nil, nil) }()
		select {
		case report := <-done:
			if want := expected(root, "alice:/home/alice"); !reflect.DeepEqual(report.Homes, want) {
				t.Errorf("linked=%t: homes %v, want %v", linked, report.Homes, want)
			}
		case <-time.After(10 * time.Second):
			// Opening the pipe for writing lets the blocked read end.
			if writer, err := os.OpenFile(pipe, os.O_WRONLY, 0); err == nil {
				_ = writer.Close()
			}
			t.Fatalf("linked=%t: Find waited on the named pipe", linked)
		}
	}
}

// TestALiveRunTakesAPasswdHomeAsWritten has getent name a home with a
// trailing slash: a live run keeps the field as the password database wrote
// it, where an image run removes its empty segments.
func TestALiveRunTakesAPasswdHomeAsWritten(t *testing.T) {
	root := tree{dirs: []string{"srv/getent"}}.build(t)
	getent := func(_ context.Context, name string, _ ...string) ([]byte, error) {
		if name == "getent" {
			return []byte("viagetent:x:1:1::/srv//getent/:/bin/sh\n"), nil
		}
		return nil, errors.New("not found")
	}
	got := Find(context.Background(), true, root, "", getent, nil, nil, nil).Homes
	if want := []Home{{"viagetent", root + "/srv//getent/"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("Find = %v, want %v", got, want)
	}
}
