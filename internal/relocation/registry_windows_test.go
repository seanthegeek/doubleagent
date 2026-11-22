package relocation

import (
	"crypto/rand"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestSystemRegistryStrings(t *testing.T) {
	cases := []struct {
		name, subkey string
		wantPath     bool
	}{
		{"the system environment key lists Path and reads without a reason",
			systemEnvironmentKey, true},
		{"a missing key gives no names and no reason",
			`SOFTWARE\doubleagent-test-no-such-key\Environment`, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := SystemRegistry().Strings(LocalMachine, test.subkey)
			if got.Reason != "" {
				t.Errorf("reason = %q, want none", got.Reason)
			}
			if hasPath := slices.Contains(got.Names, "Path"); hasPath != test.wantPath {
				t.Errorf("Path among the names = %v, want %v; names %q", hasPath, test.wantPath,
					got.Names)
			}
			if !test.wantPath && (len(got.Names) != 0 || len(got.Text) != 0) {
				t.Errorf("a missing key gave names %q and text %q", got.Names, got.Text)
			}
		})
	}
}

func TestSystemRegistryReadsTheTypeOfEachValue(t *testing.T) {
	got := SystemRegistry().Strings(LocalMachine, systemEnvironmentKey)
	// Path is a string on every install, REG_EXPAND_SZ by default but REG_SZ
	// on some images (the CI runner's), and OS is the REG_SZ Windows_NT.
	if got.Text["Path"] == "" {
		t.Errorf("Path text is empty, want the search path")
	}
	if text, ok := got.Text["OS"]; !ok || got.Expandable["OS"] || text != "Windows_NT" {
		t.Errorf("OS = %q (present %v, expandable %v), want the REG_SZ Windows_NT", text, ok,
			got.Expandable["OS"])
	}
}

func TestSystemRegistryListsLoadedHives(t *testing.T) {
	// HKEY_USERS always holds the hive of the local system account, S-1-5-18.
	if hives := SystemRegistry().LoadedHives(); !slices.Contains(hives, "S-1-5-18") {
		t.Errorf("loaded hives = %q, want S-1-5-18 among them", hives)
	}
}

// testKey creates a throwaway key under HKEY_CURRENT_USER\Software, deleted
// when the test ends, and returns it with its path under HKEY_USERS.
func testKey(t *testing.T) (registry.Key, string) {
	t.Helper()
	subkey := `Software\doubleagent-test-` + rand.Text()
	key, _, err := registry.CreateKey(registry.CURRENT_USER, subkey, registry.SET_VALUE)
	if err != nil {
		t.Fatalf("create HKCU\\%s: %v", subkey, err)
	}
	t.Cleanup(func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, subkey); err != nil {
			t.Errorf("delete HKCU\\%s: %v", subkey, err)
		}
	})
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("read the token user: %v", err)
	}
	return key, user.User.Sid.String() + `\` + subkey
}

func TestSystemRegistryKeepsAnEmbeddedNUL(t *testing.T) {
	key, usersPath := testKey(t)
	// SetStringValue refuses a NUL, so the value is written through
	// RegSetValueExW: C:\a, a NUL, junk, and the terminating NUL.
	setStringData(t, key, "CODEX_HOME", "C:\\a\x00junk\x00")
	got := SystemRegistry().Strings(UsersRoot, usersPath)
	if got.Reason != "" || !slices.Equal(got.Names, []string{"CODEX_HOME"}) {
		t.Fatalf("names %q, reason %q; want CODEX_HOME and no reason", got.Names, got.Reason)
	}
	if text := got.Text["CODEX_HOME"]; text != "C:\\a\x00junk" {
		t.Errorf("CODEX_HOME = %q, want the text through junk with its NUL", text)
	}
	if value, reason := acceptRegistryValue(got.Text["CODEX_HOME"]); reason != "holds a control character" {
		t.Errorf("accepted %q (reason %q), want it refused as holding a control character",
			value, reason)
	}
}

func TestSystemRegistryCutsAValueNameAtItsNUL(t *testing.T) {
	key, usersPath := testKey(t)
	if err := key.SetStringValue("CODEX_HOME", `C:\real`); err != nil {
		t.Fatal(err)
	}
	// No Win32 call takes a counted value name, so the second value, named
	// CODEX_HOME, a NUL and x, is written through NtSetValueKey. Windows
	// PowerShell 5.1's GetValueNames ends a name at its first NUL, so it
	// lists CODEX_HOME twice; one hint is kept per source.
	setCountedName(t, key, "CODEX_HOME\x00x", `C:\other`)
	got := SystemRegistry().Strings(UsersRoot, usersPath)
	wantNames := []string{"CODEX_HOME", "CODEX_HOME"}
	if got.Reason != "" || !slices.Equal(got.Names, wantNames) {
		t.Fatalf("names %q, reason %q; want %q and no reason", got.Names, got.Reason, wantNames)
	}
	log := &lineLog{}
	fake := fakeRegistry{keys: map[registryKey]RegistryStrings{
		{LocalMachine, systemEnvironmentKey}: got}}
	if err := registrySearch(t, fake, nil, nil, log).registryPass(); err != nil {
		t.Fatal(err)
	}
	if hints := len(log.lines) - 1; hints != 1 || !strings.HasSuffix(log.lines[1], `= C:\real`) {
		t.Errorf("%d hints, want one for the source both names give, of the real value; log %q",
			hints, log.lines)
	}
}

func TestSystemRegistryReadsPastAListedValueThatCannotBeRead(t *testing.T) {
	key, usersPath := testKey(t)
	// AAA, a NUL and x is listed as AAA, a name no value has, so reading it
	// fails as missing.
	setCountedName(t, key, "AAA\x00x", `C:\planted`)
	if err := key.SetStringValue("CODEX_HOME", `C:\real`); err != nil {
		t.Fatal(err)
	}
	got := SystemRegistry().Strings(UsersRoot, usersPath)
	if got.Reason != "" {
		t.Fatalf("reason %q, want none: one unreadable value must not hide the key", got.Reason)
	}
	if text := got.Text["CODEX_HOME"]; text != `C:\real` {
		t.Errorf("CODEX_HOME = %q, want C:\\real; names %q", text, got.Names)
	}
	if _, read := got.Text["AAA"]; read || !slices.Contains(got.Names, "AAA") || !got.Unreadable["AAA"] {
		t.Errorf("names %q, text %q, unreadable %v; want AAA listed, unreadable, without text",
			got.Names, got.Text, got.Unreadable)
	}
}

// setCountedName writes text as the REG_SZ value under key whose counted
// name is name, a NUL in it included, through NtSetValueKey.
func setCountedName(t *testing.T, key registry.Key, name, text string) {
	t.Helper()
	units := utf16.Encode([]rune(name))
	counted := windows.NTUnicodeString{Length: uint16(2 * len(units)),
		MaximumLength: uint16(2 * len(units)), Buffer: &units[0]}
	data := utf16.Encode([]rune(text + "\x00"))
	setValue := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtSetValueKey")
	status, _, _ := setValue.Call(uintptr(key), uintptr(unsafe.Pointer(&counted)), 0,
		uintptr(registry.SZ), uintptr(unsafe.Pointer(&data[0])), uintptr(2*len(data)))
	if status != 0 {
		t.Fatalf("NtSetValueKey %q: %v", name, windows.NTStatus(status))
	}
}

// setStringData writes text, terminator included, as the REG_SZ value name
// under key, as RegSetValueExW stores it.
func setStringData(t *testing.T, key registry.Key, name, text string) {
	t.Helper()
	units := utf16.Encode([]rune(text))
	data := make([]byte, 2*len(units))
	for i, unit := range units {
		data[2*i], data[2*i+1] = byte(unit), byte(unit>>8)
	}
	namePointer, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	setValue := windows.NewLazySystemDLL("advapi32.dll").NewProc("RegSetValueExW")
	status, _, _ := setValue.Call(uintptr(key), uintptr(unsafe.Pointer(namePointer)), 0,
		uintptr(registry.SZ), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	if status != 0 {
		t.Fatalf("RegSetValueExW %s: %v", name, windows.Errno(status))
	}
}
