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
			got := SystemRegistry().Strings(localMachine, test.subkey)
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
	got := SystemRegistry().Strings(localMachine, systemEnvironmentKey)
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
	got := SystemRegistry().Strings(usersRoot, usersPath)
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
	// CODEX_HOME, a NUL and x, is written through NtSetValueKey. The value
	// listing ends a name at its first NUL, so it lists CODEX_HOME twice.
	setCountedName(t, key, "CODEX_HOME\x00x", `C:\other`)
	got := SystemRegistry().Strings(usersRoot, usersPath)
	wantNames := []string{"CODEX_HOME", "CODEX_HOME"}
	if got.Reason != "" || !slices.Equal(got.Names, wantNames) {
		t.Fatalf("names %q, reason %q; want %q and no reason", got.Names, got.Reason, wantNames)
	}
	log := &lineLog{}
	fake := fakeRegistry{keys: map[registryKey]RegistryStrings{
		{localMachine, systemEnvironmentKey}: got}}
	if err := registrySearch(t, fake, nil, nil, log).registryPass(); err != nil {
		t.Fatal(err)
	}
	if hints := len(log.lines) - 1; hints != 1 ||
		!strings.HasSuffix(log.lines[1], "value not resolved: "+reasonRepeated) {
		t.Errorf("%d hints, want one without a value, as the name is listed twice; log %q",
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
	got := SystemRegistry().Strings(usersRoot, usersPath)
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

// countedName returns name as the counted string the native registry calls
// take, a NUL in it included.
func countedName(name string) *windows.NTUnicodeString {
	units := utf16.Encode([]rune(name))
	return &windows.NTUnicodeString{Length: uint16(2 * len(units)),
		MaximumLength: uint16(2 * len(units)), Buffer: &units[0]}
}

// setCountedName writes text as the REG_SZ value under key whose counted
// name is name, a NUL in it included, through NtSetValueKey.
func setCountedName(t *testing.T, key registry.Key, name, text string) {
	t.Helper()
	data := utf16.Encode([]rune(text + "\x00"))
	setValue := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtSetValueKey")
	status, _, _ := setValue.Call(uintptr(key), uintptr(unsafe.Pointer(countedName(name))), 0,
		uintptr(registry.SZ), uintptr(unsafe.Pointer(&data[0])), uintptr(2*len(data)))
	if status != 0 {
		t.Fatalf("NtSetValueKey %q: %v", name, windows.NTStatus(status))
	}
}

// deleteCountedName deletes the value under key whose counted name is name
// through NtDeleteValueKey, since no Win32 call can name a value whose name
// holds a NUL, and returns the call's status.
func deleteCountedName(key registry.Key, name string) windows.NTStatus {
	deleteValue := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtDeleteValueKey")
	status, _, _ := deleteValue.Call(uintptr(key), uintptr(unsafe.Pointer(countedName(name))))
	return windows.NTStatus(status)
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

// cutTestName is the NUL-named value TestLogonModelGivesNoValueToANameListedTwice
// plants, which the listing gives as DA_TEST_N1.
const cutTestName = "DA_TEST_N1\x00x"

// userEnvironmentKey opens the running user's HKCU\Environment for writing,
// closed when the test ends, with any DA_TEST_ value of an earlier run
// removed; the test is skipped when the key cannot be opened.
func userEnvironmentKey(t *testing.T) registry.Key {
	t.Helper()
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment",
		registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		t.Skipf("open HKCU\\Environment for writing: %v", err)
	}
	t.Cleanup(func() { key.Close() })
	removeLeftovers(t, key)
	return key
}

func TestLogonModelMatchesTheBlockWindowsBuilds(t *testing.T) {
	key := userEnvironmentKey(t)
	values := []struct {
		name, text string
		expandable bool
	}{
		{"DA_TEST_P1", `C:\p1`, false},
		{"DA_TEST_E0", `%APPDATA%\a|%LOCALAPPDATA%\l`, true},
		{"DA_TEST_E1", `%DA_TEST_P2%\later plain`, true},
		{"DA_TEST_E2", `%DA_TEST_E3%\later expandable`, true},
		{"DA_TEST_E3", `%USERPROFILE%\x`, true},
		{"DA_TEST_P2", `%SystemRoot%\plain`, false},
		{"DA_TEST_E4", `%SystemRoot%\s`, true},
		{"DA_TEST_E5", `%DA_TEST_NOPE%\u`, true},
		{"DA_TEST_E6", `%DA_TEST_P1%%DA_TEST_E1%`, true},
	}
	for _, value := range values {
		write := key.SetStringValue
		if value.expandable {
			write = key.SetExpandStringValue
		}
		if err := write(value.name, value.text); err != nil {
			t.Fatalf("write HKCU\\Environment\\%s: %v", value.name, err)
		}
		t.Cleanup(func() {
			if err := key.DeleteValue(value.name); err != nil {
				t.Errorf("delete HKCU\\Environment\\%s: %v", value.name, err)
			}
		})
	}
	model, block := logonModel(t), environmentBlock(t)
	for _, value := range values {
		folded := lowerASCII(value.name)
		if _, ok := block[folded]; !ok {
			t.Errorf("%s: missing from the block Windows built", value.name)
		}
		modelled, ok := model.lookup(value.name)
		if !ok {
			t.Errorf("%s: missing from the model", value.name)
		}
		if modelled != block[folded] {
			t.Errorf("%s: model %q, Windows %q", value.name, modelled, block[folded])
		}
	}
}

// removeLeftovers deletes every DA_TEST_ value of key, which a run that was
// killed before its cleanup left behind, cutTestName first, since its listed
// name is not one DeleteValue can remove.
func removeLeftovers(t *testing.T, key registry.Key) {
	t.Helper()
	status := deleteCountedName(key, cutTestName)
	if status != 0 && status != windows.STATUS_OBJECT_NAME_NOT_FOUND {
		t.Fatalf("delete leftover HKCU\\Environment\\%q: %v", cutTestName, status)
	}
	names, err := key.ReadValueNames(-1)
	if err != nil {
		t.Fatalf("list HKCU\\Environment: %v", err)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "DA_TEST_") {
			if err := key.DeleteValue(name); err != nil {
				t.Fatalf("delete leftover HKCU\\Environment\\%s: %v", name, err)
			}
		}
	}
}

// logonModel returns the variables the registry model gives a new logon of
// the user running the test, read from this host's registry.
func logonModel(t *testing.T) *environment {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("read the token user: %v", err)
	}
	securityID := user.User.Sid.String()
	host := SystemRegistry()
	system := readStrings(host, localMachine, systemEnvironmentKey)
	names := systemNames(host, system.Text)
	variables := newEnvironment(names)
	variables.readKey(system, false)
	variables = variables.forUser(host, securityID, names)
	variables.readKey(readStrings(host, usersRoot, securityID+`\Environment`), true)
	return variables
}

// plantListedTwice writes DA_TEST_N1 = C:\plain and cutTestName = C:\cut to
// key, both deleted when the test ends.
func plantListedTwice(t *testing.T, key registry.Key) {
	t.Helper()
	if err := key.SetStringValue("DA_TEST_N1", `C:\plain`); err != nil {
		t.Fatalf("write HKCU\\Environment\\DA_TEST_N1: %v", err)
	}
	t.Cleanup(func() {
		if err := key.DeleteValue("DA_TEST_N1"); err != nil {
			t.Errorf("delete HKCU\\Environment\\DA_TEST_N1: %v", err)
		}
	})
	setCountedName(t, key, cutTestName, `C:\cut`)
	t.Cleanup(func() {
		if status := deleteCountedName(key, cutTestName); status != 0 {
			t.Errorf("delete HKCU\\Environment\\%q: %v", cutTestName, status)
		}
	})
}

func TestLogonModelGivesNoValueToANameListedTwice(t *testing.T) {
	plantListedTwice(t, userEnvironmentKey(t))
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("read the token user: %v", err)
	}
	values := readStrings(SystemRegistry(), usersRoot, user.User.Sid.String()+`\Environment`)
	count := 0
	for _, name := range values.Names {
		if name == "DA_TEST_N1" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("the reader lists DA_TEST_N1 %d times, want 2; names %q", count, values.Names)
	}
	set, _ := newEnvironment(nil).readKey(values, true)
	if value, reason := registryValue(values, set, "DA_TEST_N1"); value != "" ||
		reason != reasonRepeated {
		t.Errorf("value %q, reason %q; want no value and %q", value, reason, reasonRepeated)
	}
	if modelled, ok := logonModel(t).lookup("DA_TEST_N1"); ok {
		t.Errorf("the model set DA_TEST_N1 = %q, want it unset", modelled)
	}
	// Windows sets the variable from one of the two values; which one is
	// what the model does not guess.
	if got := environmentBlock(t)["da_test_n1"]; got != `C:\plain` && got != `C:\cut` {
		t.Errorf("Windows gave DA_TEST_N1 = %q, want one of the two planted values", got)
	}
}

// environmentBlock returns the block Windows builds for a new logon of the
// user running the test, read from the registry now, each value under its
// name with ASCII letters lowered.
func environmentBlock(t *testing.T) map[string]string {
	t.Helper()
	lines, err := windows.GetCurrentProcessToken().Environ(false)
	if err != nil {
		t.Fatalf("build the environment block: %v", err)
	}
	block := map[string]string{}
	for _, line := range lines {
		name, text, _ := strings.Cut(line, "=")
		block[lowerASCII(name)] = text
	}
	return block
}
