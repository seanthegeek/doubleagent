package relocation

import (
	"context"
	"maps"
	"slices"
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
// order; a name ending in `*` is REG_EXPAND_SZ, and one ending in `#` is a
// value of another type, its text ignored.
func stringValues(pairs ...string) RegistryStrings {
	values := RegistryStrings{Text: map[string]string{}, Expandable: map[string]bool{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		name := strings.TrimRight(pairs[i], "*#")
		values.Names = append(values.Names, name)
		if strings.HasSuffix(pairs[i], "#") {
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
func registrySearch(t *testing.T, registry Registry, profiles []Profile, homes []users.Home,
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
		{"only the first % is looked at", `C:\50% of %A%`, "", "holds a %"},
		{"a doubled % starts no reference", `%%A%`, "", "holds a %"},
		{"a digit starts no reference", `%1%`, "", "holds a %"},
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

func TestExpandRegistryValue(t *testing.T) {
	selfLong := "%A%" + strings.Repeat("y", expansionLimit-3)
	tests := []struct {
		name, text string
		tables     []map[string]string
		want       string
	}{
		{"a reference is replaced", `%A%\x`, []map[string]string{{"A": `C:\a`}}, `C:\a\x`},
		{"names are looked up in any case", `%userprofile%`,
			[]map[string]string{{"USERPROFILE": `C:\u`}}, `C:\u`},
		{"the first table holding the name wins", `%A%`,
			[]map[string]string{{"B": "1"}, {"a": "2"}, {"A": "3"}}, "2"},
		{"an unknown reference stays", `%NOPE%\%A%`, []map[string]string{{"A": "x"}}, `%NOPE%\x`},
		{"references do not overlap", `%NOPE%A%`, []map[string]string{{"A": "x"}}, `%NOPE%A%`},
		{"a lone % before a reference is skipped", `%%A%%`, []map[string]string{{"A": "x"}},
			`%x%`},
		{"a parenthesised name is replaced", `%ProgramFiles(x86)%\p`,
			[]map[string]string{{"ProgramFiles(x86)": `C:\PF`}}, `C:\PF\p`},
		{"a replacement is expanded on the next pass", `%A%`,
			[]map[string]string{{"A": `%B%\a`, "B": "C:"}}, `C:\a`},
		{"a self reference stops after eight passes", `%A%`,
			[]map[string]string{{"A": "%A%x"}}, "%A%xxxxxxxx"},
		{"no pass starts on text over 32767 bytes", `%A%`, []map[string]string{{"A": selfLong}},
			"%A%" + strings.Repeat("y", 2*(expansionLimit-3))},
		{"an empty replacement counts as a change", `%A%%B%`,
			[]map[string]string{{"A": "", "B": "%A%z"}}, "z"},
		{"text without references is unchanged", `C:\x`, nil, `C:\x`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := expandRegistryValue(test.text, test.tables); got != test.want {
				t.Errorf("expandRegistryValue(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestSystemNames(t *testing.T) {
	versionKey := registryKey{LocalMachine, windowsVersionKey}
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
	defaults := map[string]string{"USERPROFILE": `C:\Users\alice`, "HOMEDRIVE": "C:",
		"HOMEPATH": `\Users\alice`, "APPDATA": `C:\Users\alice\AppData\Roaming`,
		"LOCALAPPDATA": `C:\Users\alice\AppData\Local`}
	overridden := maps.Clone(defaults)
	overridden["APPDATA"], overridden["LOCALAPPDATA"] = `%USERPROFILE%\Roam`, `E:\local`
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
		{"the shell folders override AppData raw",
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
				{LocalMachine, profileListKey + securityID}:     test.entry,
				{UsersRoot, securityID + `\` + shellFoldersKey}: test.folders,
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
			{LocalMachine, systemEnvironmentKey}: stringValues("Path*", `%SystemRoot%\x`,
				"copilot_home", `D:\cp\`, "CODEX_HOME*", `%SystemDrive%\codex`,
				"CLAUDE_CONFIG_DIR#", ""),
			{LocalMachine, windowsVersionKey}: stringValues("SystemRoot", `C:\Windows`),
			{LocalMachine, profileListKey + alice}: stringValues("ProfileImagePath*",
				`%SystemDrive%\Users\alice`),
			{UsersRoot, alice + `\Environment`}: stringValues("CODEX_HOME*",
				`%USERPROFILE%\.codex`, "CLAUDE_CONFIG_DIR*", `%APPDATA%\claude`),
			{UsersRoot, alice + `\` + shellFoldersKey}: stringValues("AppData", `E:\roam`),
			{UsersRoot, aliceAgain + `\Environment`}:   stringValues("CODEX_HOME", `C:\x`),
			{UsersRoot, `S-1-12-1-5\Environment`}:      {Reason: "access denied"},
			{UsersRoot, `S-1-5-18\Environment`}:        stringValues("CODEX_HOME", `C:\s`),
			{UsersRoot, `S-1-5-21-1-1004\Environment`}: stringValues("CODEX_HOME", `C:\d`),
		},
		loaded: []string{".DEFAULT", alice, "s-1-5-21-9-1001", "S-1-12-1-5", "S-1-5-18",
			"S-1-5-21-1-1004"},
	}
}

func TestRegistryPassReadsTheLiveRegistry(t *testing.T) {
	profiles := []Profile{
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
			`CLAUDE_CONFIG_DIR (claude-code) = E:\roam\claude`,
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
		{LocalMachine, systemEnvironmentKey}: stringValues("CODEX_\u017fQLITE_HOME", `C:\s`,
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
		{LocalMachine, systemEnvironmentKey}: {Reason: "access denied"},
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
