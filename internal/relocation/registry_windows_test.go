package relocation

import (
	"crypto/rand"
	"slices"
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

func TestSystemRegistryKeepsAnEmbeddedNUL(t *testing.T) {
	subkey := `Software\doubleagent-test-` + rand.Text()
	key, _, err := registry.CreateKey(registry.CURRENT_USER, subkey, registry.SET_VALUE)
	if err != nil {
		t.Fatalf("create HKCU\\%s: %v", subkey, err)
	}
	t.Cleanup(func() {
		if err := registry.DeleteKey(registry.CURRENT_USER, subkey); err != nil {
			t.Errorf("delete HKCU\\%s: %v", subkey, err)
		}
	})
	defer key.Close()
	// SetStringValue refuses a NUL, so the value is written through
	// RegSetValueExW: C:\a, a NUL, junk, and the terminating NUL.
	setStringData(t, key, "CODEX_HOME", "C:\\a\x00junk\x00")
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("read the token user: %v", err)
	}
	got := SystemRegistry().Strings(UsersRoot, user.User.Sid.String()+`\`+subkey)
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
