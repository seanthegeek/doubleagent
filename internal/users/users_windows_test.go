package users

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// fakeProfiles is a profileReader that returns a fixed list.
type fakeProfiles []Profile

func (fake fakeProfiles) Profiles() []Profile { return fake }

// writeTree creates the directories and empty files under a fresh root and
// returns the root in canonical form, spelled as the disk holds it, so
// respelled profile paths keep it as their prefix.
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
	return fsmeta.OnDiskPath("", syspath.FromNative(root))
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
			name:     "a ProfileList path in another case takes the disk's spelling and is one home",
			dirs:     []string{"Users/alice"},
			profiles: []string{"S-1-5-21-1=Users/ALICE"},
			homes:    []string{"ALICE:/Users/alice"},
		},
		{
			name:     "a ProfileList path differing in an accented letter's case is one home",
			dirs:     []string{"Users/\u00e9mile"},
			profiles: []string{"S-1-5-21-1=Users/\u00c9mile"},
			homes:    []string{"\u00c9mile:/Users/\u00e9mile"},
		},
		{
			name:  "dotless-i img beside img is a second home, as NTFS keeps the two apart",
			dirs:  []string{"Users/img", "Users/\u0131mg"},
			image: true,
			homes: []string{"img:/Users/img", "\u0131mg:/Users/\u0131mg"},
		},
		{
			name:   "the -u list ignores the case of an accented letter",
			dirs:   []string{"Users/\u00e9mile", "Users/bob"},
			filter: "\u00c9MILE",
			homes:  []string{"\u00e9mile:/Users/\u00e9mile"},
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
			if testCase.image {
				return
			}
			var wantProfiles []Profile
			for _, profile := range reader.(fakeProfiles) {
				profile.Path = fsmeta.OnDiskPath("", profile.Path)
				wantProfiles = append(wantProfiles, profile)
			}
			if !reflect.DeepEqual(report.Profiles, wantProfiles) {
				t.Errorf("Profiles = %v, want every entry of the reader in order, respelled",
					report.Profiles)
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

func TestFindJudgesALinkByItsOwnAttributes(t *testing.T) {
	cases := []struct {
		name, target string
		homes        []Home
	}{
		{"a junction to a directory that is gone is a home", "absent",
			[]Home{{"link", "/Users/link"}}},
		{"a junction to a directory is a home", "real", []Home{{"link", "/Users/link"}}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeTree(t, []string{"Users", "real"}, nil)
			makeJunction(t, root+"/"+test.target, root+"/Users/link")
			report := find(nil, root, "")
			var homes []Home
			for _, home := range report.Homes {
				homes = append(homes, Home{home.User, strings.TrimPrefix(home.Path, root)})
			}
			if !reflect.DeepEqual(homes, test.homes) || len(report.Skipped) != 0 {
				t.Errorf("Homes = %v, Skipped = %v, want %v and nothing skipped", homes,
					report.Skipped, test.homes)
			}
		})
	}
}

// allowUnprivilegedCreate is SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE,
// which golang.org/x/sys does not define: it lets an account in developer
// mode create a symlink without the privilege.
const allowUnprivilegedCreate = 0x2

func TestFindDropsAFileSymlinkToADirectory(t *testing.T) {
	root := writeTree(t, []string{"Users", "real"}, nil)
	// A symlink made without the directory flag is a file to File.Exists,
	// whatever it leads to, so Get-UserHomes drops it as a file.
	link := filepath.Join(syspath.ToNative(root), "Users", "link")
	err := windows.CreateSymbolicLink(utf16Pointer(t, link),
		utf16Pointer(t, filepath.Join(syspath.ToNative(root), "real")),
		allowUnprivilegedCreate)
	if err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	if report := find(nil, root, ""); len(report.Homes) != 0 || len(report.Skipped) != 0 {
		t.Errorf("Homes = %v, Skipped = %v, want neither", report.Homes, report.Skipped)
	}
}

// utf16Pointer returns text as a NUL-terminated UTF-16 string.
func utf16Pointer(t *testing.T, text string) *uint16 {
	t.Helper()
	pointer, err := windows.UTF16PtrFromString(text)
	if err != nil {
		t.Fatal(err)
	}
	return pointer
}

// makeJunction makes link a junction to the directory target, both
// canonical, by setting on a new empty directory the mount-point reparse
// data mklink /J writes. target need not exist.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Mkdir(syspath.ToNative(link), 0o755); err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(utf16Pointer(t, syspath.ToNative(link)), windows.GENERIC_WRITE,
		0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	native := syspath.ToNative(target)
	data := mountPointData(`\??\`+native+`\`, native)
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// mountPointData lays out a REPARSE_DATA_BUFFER holding a mount-point
// (junction) reparse buffer: the tag, the data length, a reserved word, the
// offsets and lengths in bytes of the two names, then the substitute name
// the system follows and the print name shown to people, each ending in NUL.
func mountPointData(substitute, printName string) []byte {
	substituteText := utf16.Encode([]rune(substitute))
	printText := utf16.Encode([]rune(printName))
	names := make([]uint16, 0, len(substituteText)+len(printText)+2)
	names = append(append(names, substituteText...), 0)
	names = append(append(names, printText...), 0)
	substituteBytes := uint16(2 * len(substituteText))
	data := binary.LittleEndian.AppendUint32(nil, windows.IO_REPARSE_TAG_MOUNT_POINT)
	data = binary.LittleEndian.AppendUint16(data, uint16(8+2*len(names)))
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes+2)
	data = binary.LittleEndian.AppendUint16(data, uint16(2*len(printText)))
	for _, unit := range names {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
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
	cases := []struct {
		name   string
		denied windows.ACCESS_MASK
	}{
		{"a home that cannot be listed is still a home", windows.FILE_LIST_DIRECTORY},
		// NTFS lets whoever may list a directory read its entries'
		// attributes, so even a deny on reading them leaves the entry a
		// home, never a skipped candidate: what skippedHome relies on.
		{"a home whose own attributes are denied is still a home",
			windows.FILE_LIST_DIRECTORY | windows.FILE_READ_ATTRIBUTES | windows.FILE_TRAVERSE},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeTree(t, []string{"Users/locked"}, nil)
			locked := filepath.Join(syspath.ToNative(root), "Users", "locked")
			denyAccess(t, locked, test.denied)
			if _, err := os.ReadDir(locked); err == nil {
				t.Skip("the access control entry did not stop the listing")
			}
			report := find(nil, root, "")
			want := []Home{{"locked", root + "/Users/locked"}}
			if !reflect.DeepEqual(report.Homes, want) || len(report.Skipped) != 0 {
				t.Errorf("Homes = %v, Skipped = %v, want %v and no skipped entries", report.Homes,
					report.Skipped, want)
			}
		})
	}
}

// denyAccess denies the current user the rights denied on dir and restores
// the directory's access when the test ends so the temporary directory can
// go.
func denyAccess(t *testing.T, dir string, denied windows.ACCESS_MASK) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Skipf("cannot read the process token: %v", err)
	}
	entries := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: denied,
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

func TestFindReportsTheUsersFoundAndTheUnmatchedNames(t *testing.T) {
	cases := []struct {
		name, filter     string
		homes, unmatched []string
	}{
		{"an accented name in another case selects its home", "\u00c9MILE", []string{"\u00e9mile"}, nil},
		{"an ASCII name in another case selects its home", "ALICE", []string{"alice"}, nil},
		{"a name matching nothing selects nothing and is reported", "nobody,ALICE",
			[]string{"alice"}, []string{"nobody"}},
		{"no list reports nothing unmatched", "", []string{"alice", "\u00e9mile"}, nil},
		{"a skipped profile is no home for -u", "Public", nil, []string{"Public"}},
		{"a file candidate is no home for -u", "afile", nil, []string{"afile"}},
		{"a repeat in another case is reported once", "nobody,NOBODY", nil, []string{"nobody"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeTree(t, []string{"Users/alice", "Users/\u00e9mile", "Users/Public"},
				[]string{"Users/afile"})
			report := find(nil, root, test.filter)
			var homes []string
			for _, home := range report.Homes {
				homes = append(homes, home.User)
			}
			wantFound := []string{"alice", "\u00e9mile"}
			if !reflect.DeepEqual(homes, test.homes) || !reflect.DeepEqual(report.Unmatched, test.unmatched) ||
				!reflect.DeepEqual(report.Found, wantFound) {
				t.Errorf("homes %q, unmatched %q, found %q; want %q, %q, %q", homes, report.Unmatched,
					report.Found, test.homes, test.unmatched, wantFound)
			}
		})
	}
}
