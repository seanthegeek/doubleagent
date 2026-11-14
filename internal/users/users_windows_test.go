package users

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// fakeProfiles is a profileReader that returns a fixed list.
type fakeProfiles []Profile

func (fake fakeProfiles) Profiles() []Profile { return fake }

// writeTree creates the directories and empty files under a fresh root and
// returns the root in canonical form.
func writeTree(t *testing.T, dirs, files []string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(root, file), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return syspath.FromNative(root)
}

// profilesUnder turns "SID=relative/path" entries into profiles under root.
func profilesUnder(root string, entries ...string) fakeProfiles {
	var profiles fakeProfiles
	for _, entry := range entries {
		securityID, relative, _ := strings.Cut(entry, "=")
		path := root + "/" + relative
		profiles = append(profiles, Profile{securityID, leaf(path), path})
	}
	return profiles
}

func TestFind(t *testing.T) {
	cases := []struct {
		name     string
		dirs     []string
		files    []string
		profiles []string
		image    bool
		filter   string
		homes    []string
		skipped  []string
	}{
		{
			name:     "profiles come first, then the Users and home directories",
			dirs:     []string{"Users/alice", "Users/bob", "home/carol", "srv/dave"},
			profiles: []string{"S-1-5-21-1=srv/dave"},
			homes:    []string{"dave:/srv/dave", "alice:/Users/alice", "bob:/Users/bob", "carol:/home/carol"},
		},
		{
			name:     "a home repeated in another case keeps the first spelling",
			dirs:     []string{"Users/alice"},
			profiles: []string{"S-1-5-21-1=Users/ALICE"},
			homes:    []string{"ALICE:/Users/ALICE"},
		},
		{
			name:     "system and service profiles are dropped",
			dirs:     []string{"Users/Public", "Users/default", "Users/All Users", "Users/UMFD-0", "Users/DWM-2", "Users/alice"},
			profiles: []string{"S-1-5-18=Users/Default User", "S-1-5-21-1=Users/Temp.WIN"},
			homes:    []string{"alice:/Users/alice"},
		},
		{
			name:   "the -u list ignores case and empty items",
			dirs:   []string{"Users/alice", "Users/bob"},
			filter: "ALICE,,nobody",
			homes:  []string{"alice:/Users/alice"},
		},
		{
			name:   "a -u list naming nobody keeps nobody",
			dirs:   []string{"Users/alice"},
			filter: ",",
		},
		{
			name:     "a candidate that is a file is dropped without a trace",
			dirs:     []string{"Users/alice"},
			files:    []string{"Users/afile"},
			profiles: []string{"S-1-5-21-1=Users/afile"},
			homes:    []string{"alice:/Users/alice"},
		},
		{
			name:     "a profile directory that is gone is skipped once, not denied",
			profiles: []string{"S-1-5-21-1=gone/ghost", "S-1-5-21-2=gone/GHOST"},
			skipped:  []string{"gone/ghost:false"},
		},
		{
			name:  "an image looks under Users before home, and at root and var/root",
			dirs:  []string{"Users/b", "home/a", "root", "var/root", "usr/home/c", "export/home/d"},
			image: true,
			homes: []string{"b:/Users/b", "a:/home/a", "root:/root", "root:/var/root",
				"c:/usr/home/c", "d:/export/home/d"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := writeTree(t, testCase.dirs, testCase.files)
			var reader profileReader = profilesUnder(root, testCase.profiles...)
			if testCase.image {
				reader = nil
			}
			report := find(reader, root, testCase.filter)
			var homes, skipped []string
			for _, home := range report.Homes {
				homes = append(homes, home.User+":"+strings.TrimPrefix(home.Path, root))
			}
			for _, entry := range report.Skipped {
				skipped = append(skipped, strings.TrimPrefix(entry.Path, root+"/")+":"+boolText(entry.Denied))
			}
			if !reflect.DeepEqual(homes, testCase.homes) {
				t.Errorf("homes = %v, want %v", homes, testCase.homes)
			}
			if !reflect.DeepEqual(skipped, testCase.skipped) {
				t.Errorf("skipped = %v, want %v", skipped, testCase.skipped)
			}
			if !testCase.image && !reflect.DeepEqual(report.Profiles, []Profile(reader.(fakeProfiles))) {
				t.Errorf("Profiles = %v, want every entry of the reader in order", report.Profiles)
			}
		})
	}
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func TestSkippedReasonNamesTheNativePath(t *testing.T) {
	root := writeTree(t, nil, nil)
	missing := root + "/gone/ghost"
	report := find(profilesUnder(root, "S-1-5-21-1=gone/ghost"), root, "")
	if len(report.Skipped) != 1 {
		t.Fatalf("Skipped = %v, want one entry", report.Skipped)
	}
	got := report.Skipped[0]
	if got.Path != missing || got.Denied || !strings.Contains(got.Reason, syspath.ToNative(missing)) {
		t.Errorf("Skipped = %+v, want path %q, not denied, a reason naming %q",
			got, missing, syspath.ToNative(missing))
	}
}

func TestFindSkipsAHomeThatIsListedButCannotBeSeen(t *testing.T) {
	root := writeTree(t, []string{"Users"}, nil)
	link := filepath.Join(syspath.ToNative(root), "Users", "dangling")
	if err := os.Symlink(filepath.Join(syspath.ToNative(root), "absent"), link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	report := find(nil, root, "")
	want := []Skipped{{Path: root + "/Users/dangling", Reason: "access denied", Denied: true}}
	if len(report.Homes) != 0 || !reflect.DeepEqual(report.Skipped, want) {
		t.Errorf("Homes = %v, Skipped = %v, want no homes and %v", report.Homes, report.Skipped, want)
	}
}

func TestParentListsReadsADriveRoot(t *testing.T) {
	system := syspath.SystemBase("") + "/Windows"
	if !parentLists(system) {
		t.Errorf("parentLists(%q) = false, want true", system)
	}
	if parentLists(syspath.SystemBase("") + "/no-such-directory-here") {
		t.Error("parentLists of an absent name = true, want false")
	}
}

func TestFindKeepsAHomeWhoseListingIsDenied(t *testing.T) {
	root := writeTree(t, []string{"Users/locked"}, nil)
	locked := filepath.Join(syspath.ToNative(root), "Users", "locked")
	denyListing(t, locked)
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("the access control entry did not stop the listing")
	}
	report := find(nil, root, "")
	want := []Home{{"locked", root + "/Users/locked"}}
	if !reflect.DeepEqual(report.Homes, want) || len(report.Skipped) != 0 {
		t.Errorf("Homes = %v, Skipped = %v, want %v and no skipped entries", report.Homes, report.Skipped, want)
	}
}

// denyListing makes the current user unable to list dir and restores the
// directory's access when the test ends so the temporary directory can go.
func denyListing(t *testing.T, dir string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Skipf("cannot read the process token: %v", err)
	}
	entries := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.FILE_LIST_DIRECTORY,
		AccessMode:        windows.DENY_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		t.Skipf("cannot build an access control list: %v", err)
	}
	err = windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
	if err != nil {
		t.Skipf("cannot set an access control list here: %v", err)
	}
	t.Cleanup(func() {
		_ = windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, nil, nil)
	})
}

func TestRegistryProfilesAreCanonical(t *testing.T) {
	profiles := registryProfiles{}.Profiles()
	if len(profiles) == 0 {
		t.Fatal("ProfileList gave no profiles; every Windows machine has the system account's")
	}
	for _, profile := range profiles {
		if strings.Contains(profile.Path, `\`) || strings.HasSuffix(profile.Path, "/") ||
			profile.User != leaf(profile.Path) || !strings.HasPrefix(profile.SecurityID, "S-1-") {
			t.Errorf("profile %+v is not canonical", profile)
		}
	}
}

func TestCanonicalProfilePath(t *testing.T) {
	cases := map[string]string{
		`C:\Users\alice`:    "C:/Users/alice",
		`C:\Users\alice\\`:  "C:/Users/alice",
		`\\?\C:\Users\bob\`: "C:/Users/bob",
		`D:\`:               "D:",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := canonicalProfilePath(input); got != want {
				t.Errorf("canonicalProfilePath(%q) = %q, want %q", input, got, want)
			}
		})
	}
}
