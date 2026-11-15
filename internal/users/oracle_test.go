//go:build unix

package users

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// shCollection runs the sh collector in image mode over root with the -u
// list and returns the users and homes its collection.json names.
func shCollection(t *testing.T, root, usersFilter string) (names, paths []string) {
	t.Helper()
	dash, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash is not installed")
	}
	script, err := filepath.Abs("../../collectors/collect-agent-artifacts.sh")
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	args := []string{script, "-r", root, "-o", outDir, "-q", "--no-projects", "--no-docker"}
	if usersFilter != "" {
		args = append(args, "-u", usersFilter)
	}
	command := exec.Command(dash, args...)
	command.Env = append(os.Environ(), "COLLECTOR_SH=dash")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sh collector: %v\n%s", err, output)
	}
	matches, err := filepath.Glob(filepath.Join(outDir, "*.collection.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("collection.json files = %v, %v", matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Users []string `json:"users"`
		Homes []string `json:"homes"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	return summary.Users, summary.Homes
}

func TestEnumerateMatchesShCollection(t *testing.T) {
	spec := tree{
		dirs: []string{"home/alice", "home/bob", "home/.hidden", "Users/carol", "Users/Shared",
			"root", "var/root", "usr/home/dave", "srv/zed", "srv/dup", "usr", "var/empty"},
		files: []string{"home/notes", "srv/file"},
	}
	passwd := "zed:x:5:5::/srv/zed:/bin/sh\nusr:x:6:6::/usr:/bin/false\n" +
		"first:x:7:7::/srv/dup:/bin/sh\nsecond:x:8:8::/srv/dup:/bin/sh\n" +
		"file:x:9:9::/srv/file:/bin/sh\nghost:x:10:10::/srv/ghost:/bin/sh\n" +
		"empty:x:11:11:::/bin/sh\nshort:x:12:12:/srv/zed\n"
	for _, filter := range []string{"", "bob,zed,Carol", "root,second", ","} {
		t.Run("filter "+filter, func(t *testing.T) {
			root := spec.build(t)
			if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "etc/passwd"), []byte(passwd), 0o644); err != nil {
				t.Fatal(err)
			}
			var wantUsers, wantHomes []string
			for _, home := range Enumerate(false, root, filter, nil) {
				wantUsers = append(wantUsers, home.User)
				wantHomes = append(wantHomes, home.Path)
			}
			gotUsers, gotHomes := shCollection(t, root, filter)
			sameUsers := len(gotUsers) == len(wantUsers) && (len(gotUsers) == 0 || reflect.DeepEqual(gotUsers, wantUsers))
			sameHomes := len(gotHomes) == len(wantHomes) && (len(gotHomes) == 0 || reflect.DeepEqual(gotHomes, wantHomes))
			if !sameUsers || !sameHomes {
				t.Errorf("sh collector: users %v homes %v\nEnumerate: users %v homes %v",
					gotUsers, gotHomes, wantUsers, wantHomes)
			}
		})
	}
}
