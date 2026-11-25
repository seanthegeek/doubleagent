package relocation

import (
	"context"
	"maps"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// registryKey names one key of a fakeRegistry.
type registryKey struct {
	root   RegistryRoot
	subkey string
}

// fakeRegistry is a Registry answering from a table of keys and a list of
// loaded hives; a key it lacks is missing.
type fakeRegistry struct {
	keys   map[registryKey]RegistryStrings
	loaded []string
}

// Strings returns the table's key, or a missing key's empty strings.
func (registry fakeRegistry) Strings(root RegistryRoot, subkey string) RegistryStrings {
	return registry.keys[registryKey{root, subkey}]
}

// LoadedHives returns the table's loaded hives.
func (registry fakeRegistry) LoadedHives() []string { return registry.loaded }

// stringValues builds a key's strings from name and text pairs in registry
// order; a name ending in `*` is REG_EXPAND_SZ, one ending in `#` is a value
// of another type and one ending in `!` an unreadable value, their text
// ignored.
func stringValues(pairs ...string) RegistryStrings {
	values := RegistryStrings{Text: map[string]string{}, Expandable: map[string]bool{},
		Unreadable: map[string]bool{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		name := strings.TrimRight(pairs[i], "*#!")
		values.Names = append(values.Names, name)
		switch {
		case strings.HasSuffix(pairs[i], "#"):
			continue
		case strings.HasSuffix(pairs[i], "!"):
			values.Unreadable[name] = true
			continue
		}
		values.Text[name] = pairs[i+1]
		if strings.HasSuffix(pairs[i], "*") {
			values.Expandable[name] = true
		}
	}
	return values
}

// registrySearch returns a search over the real tables that reads registry for
// profiles, with homes as the run's users and log keeping the lines.
func registrySearch(t *testing.T, registry Registry, profiles []users.Profile, homes []users.Home,
	log *lineLog) *search {
	t.Helper()
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	deps := Deps{Tables: tables, Registry: registry, Profiles: profiles, Homes: homes, Log: log}
	return &search{deps: deps, names: variableNames(tables), seen: map[string]bool{}}
}

func TestAcceptRegistryValue(t *testing.T) {
	tests := []struct {
		name, text, value, reason string
	}{
		{"a drive path is kept", `C:\Users\a\.codex`, `C:\Users\a\.codex`, ""},
		{"a forward-slash drive path is kept", `d:/x/y`, `d:/x/y`, ""},
		{"trailing separators are dropped", `C:\x\/\`, `C:\x`, ""},
		{"a drive root keeps one backslash", `C:\`, `C:\`, ""},
		{"a slashed drive root becomes a backslash root", `C://`, `C:\`, ""},
		{"a dollar and a backquote are path characters", "C:\\a$b`c", "C:\\a$b`c", ""},
		{"dots inside a name are no .. segment", `C:\a..b\...`, `C:\a..b\...`, ""},
		{"a tab is a control character", "C:\\a\tb", "", "holds a control character"},
		{"a NUL is a control character", "C:\\a\x00", "", "holds a control character"},
		{"DEL is a control character", "C:\\a\x7f", "", "holds a control character"},
		{"a control character is named before a %", "%A%\n", "", "holds a control character"},
		{"an unreplaced reference is named", `%NOPE%\x`, "", "references %NOPE%"},
		{"a reference may hold parentheses", `%ProgramFiles(x86)%\x`, "",
			"references %ProgramFiles(x86)%"},
		{"a reference may hold spaces", `%My Var%\x`, "", "references %My Var%"},
		{"a reference may start with a digit", `%1%`, "", "references %1%"},
		{"the name is the text up to the next %", `C:\50% of %A%`, "", "references % of %"},
		{"a doubled % names nothing", `%%A%`, "", "holds a %"},
		{"an unclosed reference is a bare %", `C:\%A`, "", "holds a %"},
		{"a % is named before a quote", `"%A%"`, "", "references %A%"},
		{"a quote is refused", `"C:\x"`, "", "holds a quote"},
		{"a relative path is not a drive path", `x\y`, "", "not a drive path"},
		{"a UNC path is not a drive path", `\\srv\share\x`, "", "not a drive path"},
		{"a drive-relative path is not a drive path", `C:x`, "", "not a drive path"},
		{"a bare drive is not a drive path", `C:`, "", "not a drive path"},
		{"an empty value is not a drive path", "", "", "not a drive path"},
		{"a backslash .. segment is refused", `C:\a\..\b`, "", "holds a .. segment"},
		{"a slash .. segment is refused", `C:/a/..`, "", "holds a .. segment"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, reason := acceptRegistryValue(test.text)
			if value != test.value || reason != test.reason {
				t.Errorf("acceptRegistryValue(%q) = %q, %q; want %q, %q", test.text, value,
					reason, test.value, test.reason)
			}
		})
	}
}

func TestExpandReferences(t *testing.T) {
	t.Parallel()
	variables := newEnvironment(map[string]string{"A": `C:\a`, "USERPROFILE": `C:\u`,
		"My Var": `C:\mv`, "ProgramFiles(x86)": `C:\PF`, "Loop": "%Loop%x", "Long": "%Long%",
		"Fill": strings.Repeat("y", expansionLimit-4)})
	tests := []struct {
		name, text, want string
	}{
		{"a reference is replaced", `%A%\x`, `C:\a\x`},
		{"names are looked up in any ASCII case", `%userprofile%`, `C:\u`},
		{"a name may hold spaces", `%my var%\x`, `C:\mv\x`},
		{"a name may hold parentheses", `%ProgramFiles(x86)%\p`, `C:\PF\p`},
		{"an unknown name stays", `%NOPE%\x`, `%NOPE%\x`},
		{"a closing % after an unknown name opens the next reference", `%NOPE%A%`, `%NOPEC:\a`},
		{"an empty name keeps its % and the next % opens a reference", `%%A%%`, `%C:\a%`},
		{"an unclosed reference stays", `C:\%A`, `C:\%A`},
		{"a replacement is not scanned again", `%Loop%`, "%Loop%x"},
		{"a result of exactly the limit is kept", `%Fill%%A%`,
			strings.Repeat("y", expansionLimit-4) + `C:\a`},
		{"a result past the limit leaves the text as written", `%Fill%%A%z`, `%Fill%%A%z`},
		{"text without references is unchanged", `C:\x`, `C:\x`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := expandReferences(test.text, variables); got != test.want {
				t.Errorf("expandReferences(%.60q) = %.60q (%d bytes), want %.60q (%d bytes)", test.text,
					got, len(got), test.want, len(test.want))
			}
		})
	}
}

// logonVariables returns the variables a logon of alice, whose profile is
// C:\Users\alice, gets from the system and user keys, with SystemRoot
// C:\Windows and SystemDrive C:.
func logonVariables(system, user RegistryStrings) environment {
	names := map[string]string{"SystemRoot": `C:\Windows`, "SystemDrive": "C:"}
	variables := newEnvironment(names)
	variables.readKey(system, false)
	profile := fakeRegistry{keys: map[registryKey]RegistryStrings{
		{localMachine, profileListKey + "S-1-5-21-1-1001"}: stringValues("ProfileImagePath*",
			`%SystemDrive%\Users\alice`)}}
	variables = variables.forUser(profile, "S-1-5-21-1-1001", names)
	variables.readKey(user, true)
	return variables
}

// logonCase is one case of TestLogonVariables: the two keys, the variables
// the logon must have and those it must not.
type logonCase struct {
	name         string
	system, user RegistryStrings
	want         map[string]string
	absent       []string
}

// logonOrderCases are the cases of the order the keys are read in.
func logonOrderCases() []logonCase {
	return []logonCase{
		{"a plain value is never expanded", RegistryStrings{},
			stringValues("DA_P5", `%USERPROFILE%\x`), map[string]string{"DA_P5": `%USERPROFILE%\x`}, nil},
		{"a plain value listed later resolves", RegistryStrings{},
			stringValues("DA_E3*", `%DA_P2%\e`, "DA_P2", `C:\p2`),
			map[string]string{"DA_E3": `C:\p2\e`}, nil},
		{"an expandable value listed later stays, and a replacement is not scanned again",
			RegistryStrings{}, stringValues("DA_R1*", `%DA_R2%\q`, "DA_R2*", `C:\r2`,
				"DA_R3*", `%DA_R1%\z`),
			map[string]string{"DA_R1": `%DA_R2%\q`, "DA_R2": `C:\r2`, "DA_R3": `%DA_R2%\q\z`}, nil},
		{"a value naming itself gets the system value, or keeps the reference",
			stringValues("DA_S2", `C:\sys`), stringValues("DA_S2*", `%DA_S2%;y`, "DA_S1*", `%DA_S1%;y`),
			map[string]string{"DA_S2": `C:\sys;y`, "DA_S1": `%DA_S1%;y`}, nil},
		{"a user value sees the system values of both types",
			stringValues("DA_M2*", `C:\m2`, "DA_M3", `C:\m3`),
			stringValues("DA_U2*", `%DA_M2%\u`, "DA_U3*", `%DA_M3%\u`),
			map[string]string{"DA_U2": `C:\m2\u`, "DA_U3": `C:\m3\u`}, nil},
		{"a system value never sees a user value", stringValues("DA_M10*", `%DA_U1%\s`),
			stringValues("DA_U1", `C:\u1`), map[string]string{"DA_M10": `%DA_U1%\s`}, nil},
		{"a system value naming a profile name keeps the reference",
			stringValues("DA_M11*", `%USERPROFILE%\s`), RegistryStrings{},
			map[string]string{"DA_M11": `%USERPROFILE%\s`}, nil},
		{"the system names and the profile names resolve",
			stringValues("DA_M12*", `%SystemRoot%\s`),
			stringValues("DA_U4*", `%USERPROFILE%\u`, "DA_U5*", `%localappdata%\l`),
			map[string]string{"DA_M12": `C:\Windows\s`, "DA_U4": `C:\Users\alice\u`,
				"DA_U5": `C:\Users\alice\AppData\Local\l`}, nil},
		{"HOMEDRIVE, HOMEPATH and ALLUSERSPROFILE are not known", RegistryStrings{},
			stringValues("DA_U6*", `%HOMEDRIVE%%HOMEPATH%`, "DA_U7*", `%ALLUSERSPROFILE%\x`),
			map[string]string{"DA_U6": `%HOMEDRIVE%%HOMEPATH%`, "DA_U7": `%ALLUSERSPROFILE%\x`},
			[]string{"HOMEDRIVE", "HOMEPATH"}},
	}
}

// logonNameCases are the cases of how names are read and values set.
func logonNameCases() []logonCase {
	return []logonCase{
		{"a name is every character between two %, in any ASCII case", RegistryStrings{},
			stringValues("DA_My Var", `C:\mv`, "DA_N10*", `%da_my var%\x`),
			map[string]string{"DA_N10": `C:\mv\x`}, nil},
		{"a name may hold parentheses", stringValues("ProgramFiles(x86)", `C:\PF`),
			stringValues("DA_N11*", `%ProgramFiles(x86)%\p`), map[string]string{"DA_N11": `C:\PF\p`}, nil},
		{"the second % of an unknown name may open the next reference", RegistryStrings{},
			stringValues("DA_P2", `C:\p2`, "DA_N14*", `%DA_NOPE%DA_P2%`, "DA_N07*", `%%DA_P2%%`),
			map[string]string{"DA_N14": `%DA_NOPEC:\p2`, "DA_N07": `%C:\p2%`}, nil},
		{"an empty value removes a system value of its name", stringValues("DA_Z1", `C:\sys`),
			stringValues("DA_Z1", "", "DA_Z2*", `%DA_Z1%\x`), map[string]string{"DA_Z2": `%DA_Z1%\x`},
			[]string{"DA_Z1"}},
		{"an empty system value sets nothing", stringValues("DA_Z3", ""), RegistryStrings{},
			nil, []string{"DA_Z3"}},
		{"a user Path, LibPath or Os2LibPath is added after the system value",
			stringValues("LibPath", `C:\mlib;`, "Path", `C:\mp`),
			stringValues("LibPath", `C:\ulib`, "path*", `%SystemRoot%\up`, "Os2LibPath", `C:\uo`),
			map[string]string{"LibPath": `C:\mlib;C:\ulib`, "Path": `C:\mp;C:\Windows\up`,
				"Os2LibPath": `C:\uo`}, nil},
		{"another user value replaces the system value", stringValues("DA_K1", `C:\sys`),
			stringValues("da_k1", `C:\user`), map[string]string{"DA_K1": `C:\user`}, nil},
		{"a value of another type, an unreadable one or a name holding = changes nothing",
			stringValues("DA_K2", `C:\k2`, "DA_K3", `C:\k3`),
			stringValues("DA_K2#", "", "DA_K3!", "", "DA_MULTI#", "", "DA_EQ=X", `C:\eq`,
				"DA_TYPEREF*", `%DA_MULTI%`),
			map[string]string{"DA_K2": `C:\k2`, "DA_K3": `C:\k3`, "DA_TYPEREF": `%DA_MULTI%`},
			[]string{"DA_MULTI", "DA_EQ=X"}},
	}
}

func TestLogonVariables(t *testing.T) {
	t.Parallel()
	tests := append(logonOrderCases(), logonNameCases()...)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			variables := logonVariables(test.system, test.user)
			for name, want := range test.want {
				if got, ok := variables[lowerASCII(name)]; !ok || got != want {
					t.Errorf("%s = %q (set %v), want %q", name, got, ok, want)
				}
			}
			for _, name := range test.absent {
				if got, ok := variables[lowerASCII(name)]; ok {
					t.Errorf("%s = %q, want it unset", name, got)
				}
			}
		})
	}
}

func TestReadKeyBoundsEachExpansion(t *testing.T) {
	// Each value names the one before twice: unbounded, the last would be
	// 2^60 bytes. A refused expansion is set as written.
	pairs := []string{"D0", strings.Repeat("x", 1000)}
	for i := 1; i <= 60; i++ {
		previous := "%D" + strconv.Itoa(i-1) + "%"
		pairs = append(pairs, "D"+strconv.Itoa(i)+"*", previous+previous)
	}
	// Many references to one large value, each refused before it is copied.
	pairs = append(pairs, "BIG", strings.Repeat("b", 1<<20))
	for i := range 5000 {
		pairs = append(pairs, "R"+strconv.Itoa(i)+"*", "%BIG%")
	}
	values := stringValues(pairs...)
	// Not parallel, so the allocation count is this test's alone: copying
	// BIG before refusing it would allocate about 5 GB.
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	set := newEnvironment(nil).readKey(values, false)
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 64<<20 {
		t.Errorf("reading the key allocated %d bytes, want under 64 MiB", allocated)
	}
	if got := len(set["D5"]); got != 32000 {
		t.Errorf("D5 is %d bytes, want 32000", got)
	}
	if got := set["D6"]; got != "%D5%%D5%" {
		t.Errorf("D6 = %.40q, want the text as written", got)
	}
	for name, text := range set {
		if len(text) > expansionLimit && name != "BIG" {
			t.Errorf("%s is %d bytes, past the limit", name, len(text))
		}
	}
	if got := set["R0"]; got != "%BIG%" {
		t.Errorf("R0 = %.40q, want the text as written", got)
	}
}

func TestSystemNames(t *testing.T) {
	versionKey := registryKey{localMachine, windowsVersionKey}
	tests := []struct {
		name    string
		version RegistryStrings
		system  map[string]string
		want    map[string]string
	}{
		{"the drive comes from SystemRoot", stringValues("SystemRoot", `C:\Windows`), nil,
			map[string]string{"SystemRoot": `C:\Windows`, "SystemDrive": "C:"}},
		{"the system key's SystemDrive wins", stringValues("systemroot", `C:\Windows`),
			map[string]string{"systemdrive": "D:"},
			map[string]string{"SystemRoot": `C:\Windows`, "SystemDrive": "D:"}},
		{"a SystemRoot without a drive gives no drive", stringValues("SystemRoot", `\Windows`),
			nil, map[string]string{"SystemRoot": `\Windows`}},
		{"a SystemRoot of another type is unknown", stringValues("SystemRoot#", ""), nil,
			map[string]string{}},
		{"a missing version key gives nothing", RegistryStrings{}, nil, map[string]string{}},
		{"a name differing by a non-ASCII case pair is not SystemDrive",
			stringValues("SystemRoot", `C:\Windows`), map[string]string{"\u017fystemDrive": "D:"},
			map[string]string{"SystemRoot": `C:\Windows`, "SystemDrive": "C:"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := fakeRegistry{keys: map[registryKey]RegistryStrings{versionKey: test.version}}
			if got := systemNames(registry, test.system); !maps.Equal(got, test.want) {
				t.Errorf("systemNames = %v, want %v", got, test.want)
			}
		})
	}
}

func TestProfileValues(t *testing.T) {
	const securityID = "S-1-5-21-1-1001"
	names := map[string]string{"SystemDrive": "C:", "SystemRoot": `C:\Windows`}
	defaults := map[string]string{"USERPROFILE": `C:\Users\alice`,
		"APPDATA":      `C:\Users\alice\AppData\Roaming`,
		"LOCALAPPDATA": `C:\Users\alice\AppData\Local`}
	overridden := maps.Clone(defaults)
	overridden["APPDATA"], overridden["LOCALAPPDATA"] = `C:\Users\alice\Roam`, `E:\local`
	tests := []struct {
		name    string
		entry   RegistryStrings
		folders RegistryStrings
		want    map[string]string
	}{
		{"the profile path is expanded with the system names",
			stringValues("ProfileImagePath*", `%SystemDrive%\Users\alice`), RegistryStrings{},
			defaults},
		{"a plain string is expanded too and loses its trailing backslash",
			stringValues("profileimagepath", `%systemdrive%\Users\alice\`), RegistryStrings{},
			defaults},
		{"the shell folders override AppData, expanded once",
			stringValues("ProfileImagePath", `C:\Users\alice`),
			stringValues("AppData*", `%USERPROFILE%\Roam`, "Local AppData", `E:\local`),
			overridden},
		{"a profile path left with a % gives nothing",
			stringValues("ProfileImagePath", `%NOPE%\alice`), RegistryStrings{},
			map[string]string{}},
		{"a profile path that is not a drive path gives nothing",
			stringValues("ProfileImagePath", `\\srv\profiles\alice`), RegistryStrings{},
			map[string]string{}},
		{"a drive root profile path gives nothing", stringValues("ProfileImagePath", `C:\`),
			RegistryStrings{}, map[string]string{}},
		{"a profile path of another type gives nothing",
			stringValues("ProfileImagePath#", ""), RegistryStrings{}, map[string]string{}},
		{"a missing ProfileList entry gives nothing", RegistryStrings{}, RegistryStrings{},
			map[string]string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := fakeRegistry{keys: map[registryKey]RegistryStrings{
				{localMachine, profileListKey + securityID}:     test.entry,
				{usersRoot, securityID + `\` + shellFoldersKey}: test.folders,
			}}
			if got := profileValues(registry, securityID, names); !maps.Equal(got, test.want) {
				t.Errorf("profileValues = %v, want %v", got, test.want)
			}
		})
	}
}

// liveRegistry is the registry of TestRegistryPassReadsTheLiveRegistry: a
// system key, alice's hive under two security identifiers, bob's unloaded,
// carol's denied, and two loaded hives the pass must leave unread.
func liveRegistry() fakeRegistry {
	alice, aliceAgain := "S-1-5-21-1-1001", "S-1-5-21-9-1001"
	return fakeRegistry{
		keys: map[registryKey]RegistryStrings{
			{localMachine, systemEnvironmentKey}: stringValues("Path*", `%SystemRoot%\x`,
				"copilot_home", `D:\cp\`, "CODEX_HOME*", `%SystemDrive%\codex`,
				"CLAUDE_CONFIG_DIR#", ""),
			{localMachine, windowsVersionKey}: stringValues("SystemRoot", `C:\Windows`),
			{localMachine, profileListKey + alice}: stringValues("ProfileImagePath*",
				`%SystemDrive%\Users\alice`),
			{usersRoot, alice + `\Environment`}: stringValues("CODEX_HOME*",
				`%USERPROFILE%\.codex`, "CLAUDE_CONFIG_DIR*", `%APPDATA%\claude`),
			{usersRoot, alice + `\` + shellFoldersKey}: stringValues("AppData*", `%USERPROFILE%\roam`),
			{usersRoot, aliceAgain + `\Environment`}:   stringValues("CODEX_HOME", `C:\x`),
			{usersRoot, `S-1-12-1-5\Environment`}:      {Reason: "access denied"},
			{usersRoot, `S-1-5-18\Environment`}:        stringValues("CODEX_HOME", `C:\s`),
			{usersRoot, `S-1-5-21-1-1004\Environment`}: stringValues("CODEX_HOME", `C:\d`),
		},
		loaded: []string{".DEFAULT", alice, "s-1-5-21-9-1001", "S-1-12-1-5", "S-1-5-18",
			"S-1-5-21-1-1004"},
	}
}

func TestRegistryHintOfAnUnreadableValueSaysSo(t *testing.T) {
	unreadable := RegistryStrings{Names: []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR"},
		Unreadable: map[string]bool{"CODEX_HOME": true}}
	log := &lineLog{}
	fake := fakeRegistry{keys: map[registryKey]RegistryStrings{
		{localMachine, systemEnvironmentKey}: unreadable}}
	if err := registrySearch(t, fake, nil, nil, log).registryPass(); err != nil {
		t.Fatal(err)
	}
	system := `relocation hint: HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment\`
	want := []string{"Relocation hints in the registry",
		system + "CODEX_HOME names CODEX_HOME (codex-cli); value not resolved: unreadable",
		system + "CLAUDE_CONFIG_DIR names CLAUDE_CONFIG_DIR (claude-code); value not resolved: " +
			"not a string"}
	if !slices.Equal(log.lines, want) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestRegistryHintOfAReferenceToALaterExpandableValueStays(t *testing.T) {
	log := &lineLog{}
	fake := fakeRegistry{keys: map[registryKey]RegistryStrings{
		{localMachine, systemEnvironmentKey}: stringValues("CODEX_HOME*", `%LATER%\x`,
			"LATER*", `C:\later`)}}
	if err := registrySearch(t, fake, nil, nil, log).registryPass(); err != nil {
		t.Fatal(err)
	}
	want := []string{"Relocation hints in the registry", `relocation hint: HKLM\` +
		systemEnvironmentKey + `\CODEX_HOME names CODEX_HOME (codex-cli); value not resolved: ` +
		"references %LATER%"}
	if !slices.Equal(log.lines, want) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestCollectedHomeFoldsASCIIOnly(t *testing.T) {
	homes := []users.Home{{User: "img", Path: "C:/Users/img"}}
	run := registrySearch(t, fakeRegistry{}, nil, homes, &lineLog{})
	cases := []struct {
		name, path string
		want       bool
	}{
		{"a ProfileList path differing in ASCII case is the home", "c:/users/IMG", true},
		{"dotless i names another directory, as NTFS keeps it apart", "C:/Users/\u0131mg", false},
		{"the empty path is no home", "", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := run.collectedHome(test.path); got != test.want {
				t.Errorf("collectedHome(%q) = %t, want %t", test.path, got, test.want)
			}
		})
	}
}

func TestRegistryPassReadsTheLiveRegistry(t *testing.T) {
	profiles := []users.Profile{
		{SecurityID: "S-1-5-18", User: "alice", Path: "C:/Users/alice"},
		{SecurityID: "S-1-5-21-1-1001", User: "alice", Path: "C:/Users/alice"},
		{SecurityID: "S-1-5-21-1-1002", User: "bob", Path: "C:/Users/bob"},
		{SecurityID: "S-1-12-1-5", User: "carol", Path: "C:/Users/carol"},
		{SecurityID: "S-1-5-21-1-1004", User: "dave", Path: "C:/Users/dave"},
		{SecurityID: "S-1-5-21-9-1001", User: "ALICE", Path: "c:/users/ALICE"},
	}
	homes := []users.Home{{User: "alice", Path: "C:/Users/alice"},
		{User: "bob", Path: "C:/Users/bob"}, {User: "carol", Path: "C:/Users/carol"}}
	log := &lineLog{}
	run := registrySearch(t, liveRegistry(), profiles, homes, log)
	if err := run.registryPass(); err != nil {
		t.Fatal(err)
	}
	system := `relocation hint: HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment\`
	want := []string{
		"Relocation hints in the registry",
		system + `copilot_home names COPILOT_HOME (copilot-cli) = D:\cp`,
		system + `copilot_home names COPILOT_HOME (copilot-desktop) = D:\cp`,
		system + `CODEX_HOME names CODEX_HOME (codex-cli) = C:\codex`,
		system + `CLAUDE_CONFIG_DIR names CLAUDE_CONFIG_DIR (claude-code); value not resolved: ` +
			"not a string",
		`relocation hint: HKU\S-1-5-21-1-1001\Environment\CODEX_HOME names CODEX_HOME ` +
			`(codex-cli) = C:\Users\alice\.codex`,
		`relocation hint: HKU\S-1-5-21-1-1001\Environment\CLAUDE_CONFIG_DIR names ` +
			`CLAUDE_CONFIG_DIR (claude-code) = C:\Users\alice\roam\claude`,
		"registry: hive of bob not loaded",
		"registry: hive of carol not readable: access denied",
		`relocation hint: HKU\S-1-5-21-9-1001\Environment\CODEX_HOME names CODEX_HOME ` +
			`(codex-cli) = C:\x`,
	}
	if !slices.Equal(log.lines, want) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"), strings.Join(want, "\n"))
	}
	wantOutcome := RegistryOutcome{Read: true, UsersRead: []string{"alice", "ALICE"},
		UsersNotLoaded: []string{"bob"}, UsersDenied: []string{"carol"}}
	if !registryOutcomeEqual(run.result.Registry, wantOutcome) {
		t.Errorf("outcome = %+v, want %+v", run.result.Registry, wantOutcome)
	}
	first := run.result.Hints[0]
	wantFirst := Hint{Source: `HKLM\` + systemEnvironmentKey + `\copilot_home`,
		Variable: "COPILOT_HOME", Agent: "copilot-cli", Value: `D:\cp`}
	if first != wantFirst || run.result.Resolved != 6 || len(run.result.Hints) != 7 {
		t.Errorf("first hint %+v, %d resolved of %d hints", first, run.result.Resolved,
			len(run.result.Hints))
	}
}

func TestRegistryHintsFoldASCIIOnly(t *testing.T) {
	registry := fakeRegistry{keys: map[registryKey]RegistryStrings{
		{localMachine, systemEnvironmentKey}: stringValues("CODEX_\u017fQLITE_HOME", `C:\s`,
			"CRUSH_S\u212aILLS_DIR", `C:\k`, "crush_skills_dir", `C:\a`),
	}}
	log := &lineLog{}
	run := registrySearch(t, registry, nil, nil, log)
	if err := run.registryPass(); err != nil {
		t.Fatal(err)
	}
	want := []string{"Relocation hints in the registry", `relocation hint: HKLM\` +
		systemEnvironmentKey + `\crush_skills_dir names CRUSH_SKILLS_DIR (crush) = C:\a`}
	if !slices.Equal(log.lines, want) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"), strings.Join(want, "\n"))
	}
}

// registryOutcomeEqual reports whether two outcomes hold the same fields.
func registryOutcomeEqual(got, want RegistryOutcome) bool {
	return got.Read == want.Read && got.Reason == want.Reason &&
		slices.Equal(got.UsersRead, want.UsersRead) &&
		slices.Equal(got.UsersNotLoaded, want.UsersNotLoaded) &&
		slices.Equal(got.UsersDenied, want.UsersDenied)
}

func TestRegistryPassOutcomes(t *testing.T) {
	unreadable := fakeRegistry{keys: map[registryKey]RegistryStrings{
		{localMachine, systemEnvironmentKey}: {Reason: "access denied"},
	}}
	tests := []struct {
		name     string
		registry Registry
		root     string
		outcome  RegistryOutcome
		log      []string
	}{
		{"no registry reads nothing off Windows", nil, "",
			RegistryOutcome{Reason: "not Windows"}, nil},
		{"no registry is named before image mode", nil, "/img",
			RegistryOutcome{Reason: "not Windows"}, nil},
		{"an image root reads nothing", liveRegistry(), "C:/img",
			RegistryOutcome{Reason: "image mode"}, nil},
		{"an empty registry is read without a log line", fakeRegistry{}, "",
			RegistryOutcome{Read: true}, nil},
		{"an unreadable system key is logged under the header", unreadable, "",
			RegistryOutcome{Read: true}, []string{"Relocation hints in the registry",
				"registry: system environment not readable: access denied"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &lineLog{}
			run := registrySearch(t, test.registry, nil, nil, log)
			run.deps.Root = test.root
			if err := run.registryPass(); err != nil {
				t.Fatal(err)
			}
			if !registryOutcomeEqual(run.result.Registry, test.outcome) {
				t.Errorf("outcome = %+v, want %+v", run.result.Registry, test.outcome)
			}
			if !slices.Equal(log.lines, test.log) {
				t.Errorf("log = %q, want %q", log.lines, test.log)
			}
		})
	}
}

func TestSearchRecordsWhyTheRegistryWasNotRead(t *testing.T) {
	tests := []struct {
		name     string
		registry Registry
		reason   string
	}{
		{"a search without a registry says not Windows", nil, "not Windows"},
		{"a search over an image says image mode", liveRegistry(), "image mode"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := searchFixture(t)
			log := &lineLog{}
			deps := searchDeps(t, root, log, &[]string{})
			deps.Registry = test.registry
			result, err := Search(context.Background(), deps)
			if err != nil {
				t.Fatal(err)
			}
			if result.Registry.Read || result.Registry.Reason != test.reason {
				t.Errorf("registry outcome = %+v, want reason %q", result.Registry, test.reason)
			}
			if want := startupLog(root); !slices.Equal(log.lines, want) {
				t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"),
					strings.Join(want, "\n"))
			}
		})
	}
}
