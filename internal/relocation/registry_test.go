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

// fakeRegistry is a Registry answering from a table of keys, a list of
// loaded hives and a table of the subkeys of other keys; a key it lacks is
// missing.
type fakeRegistry struct {
	keys    map[registryKey]RegistryStrings
	loaded  []string
	subkeys map[registryKey][]string
}

// Strings returns the table's key, or a missing key's empty strings.
func (registry fakeRegistry) Strings(root RegistryRoot, subkey string) RegistryStrings {
	return registry.keys[registryKey{root, subkey}]
}

// SubKeys returns the loaded hives for HKEY_USERS itself, else the table's
// subkeys of the key.
func (registry fakeRegistry) SubKeys(root RegistryRoot, subkey string) []string {
	if root == usersRoot && subkey == "" {
		return registry.loaded
	}
	return registry.subkeys[registryKey{root, subkey}]
}

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
		{"a value of 4096 bytes is kept", `C:\` + strings.Repeat("a", 4093),
			`C:\` + strings.Repeat("a", 4093), ""},
		{"a value of 4097 bytes is not resolved", `C:\` + strings.Repeat("a", 4094), "",
			"longer than 4096 bytes"},
		{"a long value is refused before any other fault", `C:\` + strings.Repeat("%", 4094), "",
			"longer than 4096 bytes"},
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
			if got, _, _ := expandReferences(test.text, variables, expansionLimit); got != test.want {
				t.Errorf("expandReferences(%.60q) = %.60q (%d bytes), want %.60q (%d bytes)", test.text,
					got, len(got), test.want, len(test.want))
			}
		})
	}
}

func TestExpandReferencesNeverReplacesAnEmptyName(t *testing.T) {
	t.Parallel()
	variables := newEnvironment(map[string]string{"": "x", "a": `C:\a`})
	if got, _, _ := expandReferences(`%%a%%`, variables, expansionLimit); got != `%C:\a%` {
		t.Errorf("expandReferences = %q, want %q", got, `%C:\a%`)
	}
}

func TestExpandReferencesReturnsTheReplacementTextItCopied(t *testing.T) {
	t.Parallel()
	fill := strings.Repeat("y", expansionLimit-4)
	variables := newEnvironment(map[string]string{"A": `C:\a`, "Fill": fill})
	tests := []struct {
		name, text, want string
		allowance, cost  int
		overAllowance    bool
	}{
		{"text without references copies nothing", `C:\x`, `C:\x`, 100, 0, false},
		{"a full expansion costs every replacement", `%A%\%A%`, `C:\a\C:\a`, 100, 8, false},
		{"a refusal by the limit after one copy costs that copy", `%Fill%%Fill%`,
			`%Fill%%Fill%`, 1 << 20, len(fill), false},
		{"a refusal by the allowance before any copy costs nothing", `%A%`, `%A%`, 3, 0, true},
		{"a refusal by the allowance after one copy costs that copy", `%A%%A%`, `%A%%A%`, 6, 4,
			true},
		{"a replacement exactly filling the allowance is made", `%A%`, `C:\a`, 4, 4, false},
		{"a replacement past both the limit and the allowance is refused by the limit",
			`%Fill%%Fill%`, `%Fill%%Fill%`, len(fill) + 1, len(fill), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, cost, overAllowance := expandReferences(test.text, variables, test.allowance)
			if got != test.want || cost != test.cost || overAllowance != test.overAllowance {
				t.Errorf("expandReferences(%.40q, %d) = %.40q, %d, %t; want %.40q, %d, %t",
					test.text, test.allowance, got, cost, overAllowance, test.want, test.cost,
					test.overAllowance)
			}
		})
	}
}

// logonVariables returns the variables a logon of alice, whose profile is
// C:\Users\alice, gets from the system and user keys, with SystemRoot
// C:\Windows and SystemDrive C:.
func logonVariables(system, user RegistryStrings) *environment {
	names := map[string]string{"SystemRoot": `C:\Windows`, "SystemDrive": "C:"}
	variables := newEnvironment(names)
	variables.readKey(system, false)
	profile := fakeRegistry{keys: map[registryKey]RegistryStrings{
		{localMachine, profileListKey + "S-1-5-21-1-1001"}: stringValues("ProfileImagePath*",
			`%SystemDrive%\Users\alice`)}}
	variables = logon{system: variables, names: names}.forUser(profile, "S-1-5-21-1-1001")
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
		{"HOMEDRIVE, HOMEPATH and USERDOMAIN are not known", RegistryStrings{},
			stringValues("DA_U6*", `%HOMEDRIVE%%HOMEPATH%`, "DA_U7*", `%USERDOMAIN%\x`),
			map[string]string{"DA_U6": `%HOMEDRIVE%%HOMEPATH%`, "DA_U7": `%USERDOMAIN%\x`},
			[]string{"HOMEDRIVE", "HOMEPATH", "USERDOMAIN"}},
	}
}

// logonNameCases are the cases of how names are read and values set.
func logonNameCases() []logonCase {
	return []logonCase{
		{"a name is every character between two %, in any ASCII case", RegistryStrings{},
			stringValues("DA_My Var", `C:\mv`, "DA_N10*", `%da_my var%\x`),
			map[string]string{"DA_N10": `C:\mv\x`}, nil},
		{"a name may hold parentheses", stringValues("DA_PF(x86)", `C:\PF`),
			stringValues("DA_N11*", `%DA_PF(x86)%\p`), map[string]string{"DA_N11": `C:\PF\p`}, nil},
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
		{"a value of another type or a name holding = changes nothing",
			stringValues("DA_K2", `C:\k2`),
			stringValues("DA_K2#", "", "DA_MULTI#", "", "DA_EQ=X", `C:\eq`,
				"DA_TYPEREF*", `%DA_MULTI%`),
			map[string]string{"DA_K2": `C:\k2`, "DA_TYPEREF": `%DA_MULTI%`},
			[]string{"DA_MULTI", "DA_EQ=X"}},
		{"an unreadable value hides the system value, and a reference to it stays",
			stringValues("DA_K3", `C:\k3`), stringValues("DA_K3!", "", "DA_K3R*", `%DA_K3%\r`),
			map[string]string{"DA_K3R": `%DA_K3%\r`}, []string{"DA_K3"}},
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
				if got, ok := variables.lookup(name); !ok || got != want {
					t.Errorf("%s = %q (set %v), want %q", name, got, ok, want)
				}
			}
			for _, name := range test.absent {
				if got, ok := variables.lookup(name); ok {
					t.Errorf("%s = %q, want it unset", name, got)
				}
			}
		})
	}
}

func TestEnvironmentKeepsAnUnknownNameApartFromAnUnsetOne(t *testing.T) {
	t.Parallel()
	system := newEnvironment(map[string]string{"A": `C:\a`, "B": `C:\b`, "C": `C:\c`})
	system.forget("A")
	user := logon{system: system}.forUser(fakeRegistry{}, "S-1-5-21-1-1001")
	user.set("b", "", true)
	user.forget("C")
	user.set("D", `C:\d`, true)
	tests := []struct {
		name, variable, value string
		set, unknown          bool
	}{
		{"a name forgotten below is set to an unknown value", "a", "", true, true},
		{"a name the layer emptied is not set", "B", "", false, false},
		{"a name forgotten in the layer hides the value below", "C", "", true, true},
		{"a name set in the layer has its value", "D", `C:\d`, true, false},
		{"a name never set is not set", "E", "", false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value, set, unknown := user.resolve(test.variable)
			if value != test.value || set != test.set || unknown != test.unknown {
				t.Errorf("resolve(%s) = %q, %v, %v; want %q, %v, %v", test.variable, value, set,
					unknown, test.value, test.set, test.unknown)
			}
			if _, ok := user.lookup(test.variable); ok != (test.set && !test.unknown) {
				t.Errorf("lookup(%s) found %v", test.variable, ok)
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
	set, _ := newEnvironment(nil).readKey(values, false)
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

func TestReadKeyBoundsTheExpansionsOfOneKey(t *testing.T) {
	// 2000 values naming one 32000-byte value would each allocate 32 KB
	// unless the key's total is bounded; not parallel, so the allocation
	// count is this test's alone.
	pairs := []string{"BIG", strings.Repeat("b", 32000)}
	for i := range 2000 {
		pairs = append(pairs, "R"+strconv.Itoa(i)+"*", "%BIG%")
	}
	values := stringValues(pairs...)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	set, overBudget := newEnvironment(nil).readKey(values, false)
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 16<<20 {
		t.Errorf("reading the key allocated %d bytes, want under 16 MiB", allocated)
	}
	if !overBudget {
		t.Errorf("readKey did not report the budget passed")
	}
	if got := len(set["R0"]); got != 32000 {
		t.Errorf("R0 is %d bytes, want it expanded to 32000", got)
	}
	if got := set["R1999"]; got != "%BIG%" {
		t.Errorf("R1999 = %.40q, want the text as written", got)
	}
}

// allocatedBy returns the bytes read allocates; its caller is not parallel, so
// the count is that call's alone.
func allocatedBy(read func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	read()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// keyOf returns a key holding BIG, a plain value of size bytes, then count
// REG_EXPAND_SZ values of text, and the key's size (keyLength).
func keyOf(size, count int, text string) (RegistryStrings, int) {
	pairs := []string{"BIG", strings.Repeat("b", size)}
	for i := range count {
		pairs = append(pairs, "R"+strconv.Itoa(i)+"*", text)
	}
	return stringValues(pairs...), keyLength(pairs)
}

// entryBytes is what a test counts for each value of a key besides its name
// and text: the map entries that hold it in the key and in readKey's maps.
const entryBytes = 64

// keyLength returns the size of the key pairs holds: the bytes of its names
// and text and entryBytes for each value.
func keyLength(pairs []string) int {
	total := 0
	for _, pair := range pairs {
		total += len(pair)
	}
	return total + entryBytes*len(pairs)/2
}

func TestReadKeyMetersAReplacementCopiedBeforeARefusal(t *testing.T) {
	// Each value copies BIG once, then is refused by the limit at the second
	// BIG; charged only its short text, 50,000 values would copy 1.6 GB.
	values, _ := keyOf(32000, 50000, "%BIG%%BIG%")
	var set map[string]string
	var overBudget bool
	allocated := allocatedBy(func() { set, overBudget = newEnvironment(nil).readKey(values, false) })
	if allocated > 16<<20 {
		t.Errorf("reading the key allocated %d bytes, want under 16 MiB", allocated)
	}
	if !overBudget {
		t.Errorf("readKey did not report the budget passed")
	}
	if got := set["R49999"]; got != "%BIG%%BIG%" {
		t.Errorf("R49999 = %.40q, want the text as written", got)
	}
}

// boundShapes are the keys of TestReadKeyAllocatesWithinTheKeyAndTheBudget,
// each with its size (keyLength).
func boundShapes() map[string]func() (RegistryStrings, int) {
	chain := func() (RegistryStrings, int) {
		pairs := []string{"D0", strings.Repeat("x", 1000)}
		for i := 1; i <= 200; i++ {
			previous := "%D" + strconv.Itoa(i-1) + "%"
			pairs = append(pairs, "D"+strconv.Itoa(i)+"*", previous+previous)
		}
		return stringValues(pairs...), keyLength(pairs)
	}
	mix := func() (RegistryStrings, int) {
		pairs := []string{"BIG", strings.Repeat("b", 32000), "A", `C:\a`}
		for i := range 6000 {
			texts := []string{"%BIG%", "%BIG%%BIG%", "%A%%A%%A%", `C:\plain`, "%BIG%" +
				strings.Repeat("z", 700)}
			pairs = append(pairs, "M"+strconv.Itoa(i)+"*", texts[i%len(texts)])
		}
		return stringValues(pairs...), keyLength(pairs)
	}
	return map[string]func() (RegistryStrings, int){
		"many values naming one large value": func() (RegistryStrings, int) {
			return keyOf(32000, 20000, "%BIG%")
		},
		"many values naming one large value twice": func() (RegistryStrings, int) {
			return keyOf(32000, 20000, "%BIG%%BIG%")
		},
		"a chain of values each naming the one before twice": chain,
		"values just under the per-value limit": func() (RegistryStrings, int) {
			return keyOf(expansionLimit-3, 20000, "%BIG%xyz")
		},
		"a mix of the shapes": mix,
	}
}

func TestReadKeyAllocatesWithinTheKeyAndTheBudget(t *testing.T) {
	// Reading a key copies at most expansionBudget bytes of replacement text,
	// so its memory is a small multiple of the key's size plus the budget,
	// the multiple allowing for a builder's growth.
	for name, shape := range boundShapes() {
		t.Run(name, func(t *testing.T) {
			values, length := shape()
			allocated := allocatedBy(func() { newEnvironment(nil).readKey(values, false) })
			if bound := 4 * uint64(length+expansionBudget); allocated > bound {
				t.Errorf("reading a key of %d bytes allocated %d bytes, want at most %d", length,
					allocated, bound)
			}
		})
	}
}

func TestSetKeepsTheExistingValueWhenTheJoinPassesTheLimit(t *testing.T) {
	// A user Path joined after a 1 MiB system Path would copy it for every user.
	large := strings.Repeat("p", 1<<20)
	variables := newEnvironment(map[string]string{"Path": large})
	user := stringValues("Path", `C:\u`)
	if allocated := allocatedBy(func() { variables.readKey(user, true) }); allocated > 64<<10 {
		t.Errorf("reading the user key allocated %d bytes, want under 64 KiB", allocated)
	}
	if got, _ := variables.lookup("Path"); got != large {
		t.Errorf("Path is %d bytes, want the system value kept", len(got))
	}
	fits := strings.Repeat("p", expansionLimit-len(`;C:\u`))
	tests := []struct {
		name, existing, want string
	}{
		{"a join of exactly the limit is made", fits, fits + `;C:\u`},
		{"a join one byte past the limit is not", fits + "p", fits + "p"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			variables := newEnvironment(map[string]string{"Path": test.existing})
			variables.set("Path", `C:\u`, true)
			if got, _ := variables.lookup("Path"); got != test.want {
				t.Errorf("Path is %d bytes, want %d", len(got), len(test.want))
			}
		})
	}
}

// repeatedRegistry is the registry of the tests of a name listed more than
// once: the system and user keys given, alice's profile C:\Users\alice and
// SystemRoot C:\Windows.
func repeatedRegistry(system, user RegistryStrings) fakeRegistry {
	return fakeRegistry{keys: map[registryKey]RegistryStrings{
		{localMachine, systemEnvironmentKey}: system,
		{localMachine, windowsVersionKey}:    stringValues("SystemRoot", `C:\Windows`),
		{localMachine, profileListKey + "S-1-5-21-1-1001"}: stringValues("ProfileImagePath",
			`C:\Users\alice`),
		{usersRoot, `S-1-5-21-1-1001\Environment`}: user,
	}, loaded: []string{"S-1-5-21-1-1001"}}
}

// userHintLines returns the hints of alice's user key that a registry pass
// over repeatedRegistry logs.
func userHintLines(t *testing.T, system, user RegistryStrings) []string {
	t.Helper()
	log := &lineLog{}
	homes := []users.Home{{User: "alice", Path: `C:\Users\alice`}}
	profiles := []users.Profile{{SecurityID: "S-1-5-21-1-1001", User: "alice",
		Path: `C:\Users\alice`}}
	run := registrySearch(t, repeatedRegistry(system, user), profiles, homes, log)
	if err := run.registryPass(); err != nil {
		t.Fatal(err)
	}
	var hints []string
	for _, line := range log.lines {
		if strings.HasPrefix(line, `relocation hint: HKU\`) {
			hints = append(hints, line)
		}
	}
	return hints
}

func TestRegistryHintOfANameListedMoreThanOnceHasNoValue(t *testing.T) {
	t.Parallel()
	system := stringValues("CODEX_HOME", `C:\sys`)
	user := `relocation hint: HKU\S-1-5-21-1-1001\Environment\`
	repeated := user + "CODEX_HOME names CODEX_HOME (codex-cli); value not resolved: " +
		reasonRepeated
	reference := "; value not resolved: references %CODEX_HOME%"
	tests := []struct {
		name, first, again, want, later string
	}{
		{"two plain values", "CODEX_HOME", "CODEX_HOME", repeated, reference},
		{"a plain then an expandable value", "CODEX_HOME", "CODEX_HOME*", repeated,
			reference},
		{"an expandable then a plain value", "CODEX_HOME*", "CODEX_HOME", repeated,
			reference},
		{"two expandable values", "CODEX_HOME*", "CODEX_HOME*", repeated, reference},
		{"two listings differing in ASCII case, hinted at the first", "CODEX_HOME*",
			"codex_home", repeated, reference},
		{"one listing is a value as before", "CODEX_HOME*", "",
			user + `CODEX_HOME names CODEX_HOME (codex-cli) = C:\sys\x`, ` = C:\sys\x\c`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pairs := []string{test.first, `%CODEX_HOME%\x`}
			if test.again != "" {
				pairs = append(pairs, test.again, `C:\again`)
			}
			pairs = append(pairs, "CLAUDE_CONFIG_DIR*", `%CODEX_HOME%\c`)
			// A later value referring to a name listed more than once keeps
			// the reference, never the system value Windows replaced.
			want := []string{test.want, user + "CLAUDE_CONFIG_DIR names CLAUDE_CONFIG_DIR " +
				"(claude-code)" + test.later}
			if got := userHintLines(t, system, stringValues(pairs...)); !slices.Equal(got, want) {
				t.Errorf("user hints:\n%s\nwant:\n%s", strings.Join(got, "\n"),
					strings.Join(want, "\n"))
			}
		})
	}
}

func TestRegistryPassSetsNoNameListedMoreThanOnceInTheSystemKey(t *testing.T) {
	t.Parallel()
	system := stringValues("SystemDrive", "D:", "SystemDrive", "E:", "CODEX_HOME*",
		`%SystemDrive%\codex`, "COPILOT_HOME", `C:\one`, "COPILOT_HOME*", `C:\two`,
		"CLAUDE_CONFIG_DIR*", `%COPILOT_HOME%\c`)
	log := &lineLog{}
	if err := registrySearch(t, repeatedRegistry(system, RegistryStrings{}), nil, nil,
		log).registryPass(); err != nil {
		t.Fatal(err)
	}
	key := `relocation hint: HKLM\` + systemEnvironmentKey + `\`
	want := []string{"Relocation hints in the registry",
		key + "CODEX_HOME names CODEX_HOME (codex-cli); value not resolved: " +
			"references %SystemDrive%",
		key + "COPILOT_HOME names COPILOT_HOME (copilot-cli); value not resolved: " +
			reasonRepeated,
		key + "COPILOT_HOME names COPILOT_HOME (copilot-desktop); value not resolved: " +
			reasonRepeated,
		key + "CLAUDE_CONFIG_DIR names CLAUDE_CONFIG_DIR (claude-code); value not resolved: " +
			"references %COPILOT_HOME%"}
	if !slices.Equal(log.lines, want) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestRegistryHintOfAReferenceToAnUnknownNameStays(t *testing.T) {
	t.Parallel()
	user := `relocation hint: HKU\S-1-5-21-1-1001\Environment\`
	codex, claude := user+"CODEX_HOME names CODEX_HOME (codex-cli)",
		user+"CLAUDE_CONFIG_DIR names CLAUDE_CONFIG_DIR (claude-code)"
	unresolved := "; value not resolved: "
	tests := []struct {
		name         string
		system, user []string
		want         []string
	}{
		{"a user name listed twice hides the system value", []string{"CODEX_HOME", `C:\sys`},
			[]string{"CODEX_HOME*", `%CODEX_HOME%\x`, "CODEX_HOME", `C:\real`,
				"CLAUDE_CONFIG_DIR*", `%CODEX_HOME%\c`},
			[]string{codex + unresolved + reasonRepeated,
				claude + unresolved + "references %CODEX_HOME%"}},
		{"a user name listed twice hides the profile value", nil,
			[]string{"USERPROFILE", `D:\elsewhere`, "USERPROFILE", `D:\elsewhere`,
				"CODEX_HOME*", `%USERPROFILE%\.codex`},
			[]string{codex + unresolved + "references %USERPROFILE%"}},
		{"a user Path listed twice makes the joined Path unknown", []string{"Path", `C:\s`},
			[]string{"Path", `C:\u`, "Path", `C:\u`, "CODEX_HOME*", `%Path%\x`},
			[]string{codex + unresolved + "references %Path%"}},
		{"an unreadable user value hides the system value", []string{"CODEX_HOME", `C:\sys`},
			[]string{"CODEX_HOME!", "", "CLAUDE_CONFIG_DIR*", `%CODEX_HOME%\c`},
			[]string{codex + unresolved + reasonUnreadable,
				claude + unresolved + "references %CODEX_HOME%"}},
		{"a reference listed before the name listed twice stays too",
			[]string{"CODEX_HOME", `C:\sys`}, []string{"CLAUDE_CONFIG_DIR*", `%CODEX_HOME%\c`,
				"CODEX_HOME", `C:\x`, "CODEX_HOME*", `C:\y`},
			[]string{claude + unresolved + "references %CODEX_HOME%",
				codex + unresolved + reasonRepeated}},
		{"a system name listed twice is unknown in the user key",
			[]string{"SystemRoot", `C:\Windows`, "SystemRoot", `D:\Windows`},
			[]string{"CODEX_HOME*", `%SystemRoot%\x`},
			[]string{codex + unresolved + "references %SystemRoot%"}},
		{"an unreadable system value is unknown in the user key",
			[]string{"SystemDrive!", ""}, []string{"CODEX_HOME*", `%SystemDrive%\x`},
			[]string{codex + unresolved + "references %SystemDrive%"}},
		{"a user Path joined to an unknown system Path is unknown",
			[]string{"Path", `C:\a`, "Path", `C:\b`},
			[]string{"Path", `C:\u`, "CODEX_HOME*", `%Path%\x`},
			[]string{codex + unresolved + "references %Path%"}},
		{"a user value replaces an unknown system value",
			[]string{"DA_X", `C:\a`, "DA_X", `C:\b`},
			[]string{"DA_X", `C:\u`, "CODEX_HOME*", `%DA_X%\x`},
			[]string{codex + ` = C:\u\x`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			system := stringValues(test.system...)
			got := userHintLines(t, system, stringValues(test.user...))
			if !slices.Equal(got, test.want) {
				t.Errorf("user hints:\n%s\nwant:\n%s", strings.Join(got, "\n"),
					strings.Join(test.want, "\n"))
			}
		})
	}
}

func TestReadStringsLeavesTheRegistrysOwnTextAlone(t *testing.T) {
	t.Parallel()
	values := stringValues("A", `C:\a`, "A", `C:\b`)
	fake := fakeRegistry{keys: map[registryKey]RegistryStrings{{usersRoot, "K"}: values}}
	if got := readStrings(fake, usersRoot, "K"); len(got.Text) != 0 || !got.Repeated["a"] {
		t.Errorf("text %q, repeated %v; want no text and A repeated", got.Text, got.Repeated)
	}
	if values.Text["A"] != `C:\b` {
		t.Errorf("the registry's own text became %q", values.Text)
	}
}

func TestReadKeyGoesOnAfterAValueTheLimitRefuses(t *testing.T) {
	// After 32 copies of BIG, HUGE would pass both the per-value limit and
	// what is left of the budget; the limit refuses it, so the budget is not
	// passed and the later value is still expanded.
	pairs := []string{"BIG", strings.Repeat("b", 32000), "HUGE", strings.Repeat("h", 40000),
		"A", `C:\a`}
	for i := range 32 {
		pairs = append(pairs, "R"+strconv.Itoa(i)+"*", "%BIG%")
	}
	pairs = append(pairs, "X*", "%HUGE%", "Y*", `%A%\y`)
	set, overBudget := newEnvironment(nil).readKey(stringValues(pairs...), false)
	if overBudget {
		t.Errorf("readKey reported the budget passed")
	}
	if set["X"] != "%HUGE%" || set["Y"] != `C:\a\y` {
		t.Errorf("X = %.20q, Y = %q; want X as written and Y expanded", set["X"], set["Y"])
	}
}

func TestForUserCostsOnlyTheUsersOwnValues(t *testing.T) {
	// Copying a system environment of 50,000 values for each of 200 users
	// would allocate about 1 GB; not parallel, so the count is this test's.
	names := map[string]string{}
	systemSize := 0
	for i := range 50000 {
		name, value := "SYSTEM_VARIABLE_"+strconv.Itoa(i), `C:\system\`+strconv.Itoa(i)
		names[name] = value
		systemSize += len(name) + len(value) + entryBytes
	}
	start := logon{system: newEnvironment(names), programs: programNames(fakeRegistry{}, nil)}
	user := stringValues("CODEX_HOME*", `%SYSTEM_VARIABLE_7%\codex`)
	const users, perUser = 200, 4096
	allocated := allocatedBy(func() {
		for i := range users {
			start.forUser(fakeRegistry{}, "S-1-5-21-1-"+strconv.Itoa(i)).readKey(user, true)
		}
	})
	if bound := uint64(4 * (systemSize + users*perUser)); allocated > bound {
		t.Errorf("%d users allocated %d bytes, want at most %d", users, allocated, bound)
	}
}

func TestAUserKeyChangesOnlyThatUsersVariables(t *testing.T) {
	t.Parallel()
	system := newEnvironment(map[string]string{"DA_Z1": `C:\z1`, "DA_K1": `C:\sys`})
	first := logon{system: system}.forUser(fakeRegistry{}, "S-1-5-21-1-1001")
	set, _ := first.readKey(stringValues("DA_Z1", "", "da_k1", `C:\user`, "DA_R1*", `%DA_Z1%\x`,
		"DA_R2*", `%Da_K1%\x`), true)
	second := logon{system: system}.forUser(fakeRegistry{}, "S-1-5-21-1-1002")
	tests := []struct {
		name      string
		variables *environment
		variable  string
		want      string
		found     bool
	}{
		{"an empty user value hides the system variable from that user", first, "DA_Z1", "", false},
		{"the system still has the variable the user emptied", system, "DA_Z1", `C:\z1`, true},
		{"a second user still has the variable the first emptied", second, "da_z1", `C:\z1`, true},
		{"a user value in another case replaces the system value", first, "DA_K1", `C:\user`, true},
		{"the system keeps its value under the user's", system, "DA_K1", `C:\sys`, true},
		{"a second user keeps the system value", second, "DA_K1", `C:\sys`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := test.variables.lookup(test.variable)
			if got != test.want || found != test.found {
				t.Errorf("%s = %q (found %t), want %q (found %t)", test.variable, got, found,
					test.want, test.found)
			}
		})
	}
	if set["DA_R1"] != `%DA_Z1%\x` || set["DA_R2"] != `C:\user\x` {
		t.Errorf("DA_R1 = %q, DA_R2 = %q; want the reference kept and the user value",
			set["DA_R1"], set["DA_R2"])
	}
}

func TestForUserKeepsOneUsersValuesFromAnother(t *testing.T) {
	t.Parallel()
	system := newEnvironment(map[string]string{"SystemRoot": `C:\Windows`})
	registry := fakeRegistry{}
	first := logon{system: system}.forUser(registry, "S-1-5-21-1-1001")
	first.readKey(stringValues("ONLYFIRST", `C:\first`), true)
	second := logon{system: system}.forUser(registry, "S-1-5-21-1-1002")
	set, _ := second.readKey(stringValues("OTHER*", `%ONLYFIRST%\x`), true)
	if got := set["OTHER"]; got != `%ONLYFIRST%\x` {
		t.Errorf("OTHER = %q, want the reference kept", got)
	}
	if _, leaked := system.lookup("ONLYFIRST"); leaked {
		t.Errorf("the system environment gained ONLYFIRST")
	}
}

func TestSystemNames(t *testing.T) {
	drives := map[string]string{"SystemRoot": `C:\Windows`, "SystemDrive": "C:"}
	withNames := func(pairs ...string) map[string]string {
		names := maps.Clone(drives)
		for i := 0; i+1 < len(pairs); i += 2 {
			names[pairs[i]] = pairs[i+1]
		}
		return names
	}
	windows := stringValues("SystemRoot", `C:\Windows`)
	tests := []struct {
		name              string
		version, profiles RegistryStrings
		want              map[string]string
	}{
		{"the drive comes from SystemRoot", stringValues("systemroot", `C:\Windows`),
			RegistryStrings{}, drives},
		{"a SystemRoot without a drive gives no drive", stringValues("SystemRoot", `\Windows`),
			RegistryStrings{}, map[string]string{"SystemRoot": `\Windows`}},
		{"a SystemRoot of another type is unknown", stringValues("SystemRoot#", ""),
			RegistryStrings{}, map[string]string{}},
		{"a missing version key gives nothing", RegistryStrings{}, RegistryStrings{},
			map[string]string{}},
		{"a SystemRoot listed more than once is unknown",
			stringValues("SystemRoot", `C:\Windows`, "SystemRoot", `D:\Windows`), RegistryStrings{},
			map[string]string{}},
		{"a name differing by a non-ASCII case pair is not SystemRoot",
			stringValues("\u017fystemRoot", `C:\Windows`), RegistryStrings{}, map[string]string{}},
		{"ProgramData gives ALLUSERSPROFILE too, and Public gives PUBLIC, expanded from the drives",
			windows, stringValues("programdata*", `%SystemDrive%\ProgramData`, "Public*",
				`%SystemRoot%\Pub`),
			withNames("ProgramData", `C:\ProgramData`, "ALLUSERSPROFILE", `C:\ProgramData`,
				"PUBLIC", `C:\Windows\Pub`)},
		{"a plain source is taken as written, and an expandable one sees only the drives",
			windows, stringValues("ProgramData", `%SystemDrive%\PDS`, "Public*", `%ProgramData%\Pub`),
			withNames("ProgramData", `%SystemDrive%\PDS`, "ALLUSERSPROFILE", `%SystemDrive%\PDS`,
				"PUBLIC", `%ProgramData%\Pub`)},
		{"an empty or unreadable source sets nothing", windows,
			stringValues("ProgramData", "", "Public!", ""), drives},
		{"a source listed more than once sets nothing", windows,
			stringValues("ProgramData", `C:\a`, "ProgramData*", `C:\b`), drives},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := fakeRegistry{keys: map[registryKey]RegistryStrings{
				{localMachine, windowsVersionKey}: test.version,
				{localMachine, profilesKey}:       test.profiles}}
			if got := systemNames(registry); !maps.Equal(got, test.want) {
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
		{"a profile path listed more than once gives nothing",
			stringValues("ProfileImagePath", `C:\Users\alice`, "ProfileImagePath", `D:\x`),
			RegistryStrings{}, map[string]string{}},
		{"an AppData listed more than once keeps its default",
			stringValues("ProfileImagePath", `C:\Users\alice`),
			stringValues("AppData", `E:\r`, "AppData*", `E:\s`), defaults},
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
// system key, alice's hive under two security identifiers, the first with a
// Volatile Environment key and a session subkey, bob's unloaded, carol's
// denied, and two loaded hives the pass must leave unread.
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
			{usersRoot, alice + `\Volatile Environment`}: stringValues("AMP_HOME*",
				`%LOCALAPPDATA%\amp`),
			{usersRoot, alice + `\Volatile Environment\1`}: stringValues("CLINE_DIR*",
				`%CODEX_HOME%\cline`),
			{usersRoot, aliceAgain + `\Environment`}:   stringValues("CODEX_HOME", `C:\x`),
			{usersRoot, `S-1-12-1-5\Environment`}:      {Reason: "access denied"},
			{usersRoot, `S-1-5-18\Environment`}:        stringValues("CODEX_HOME", `C:\s`),
			{usersRoot, `S-1-5-21-1-1004\Environment`}: stringValues("CODEX_HOME", `C:\d`),
		},
		loaded: []string{".DEFAULT", alice, "s-1-5-21-9-1001", "S-1-12-1-5", "S-1-5-18",
			"S-1-5-21-1-1004"},
		subkeys: map[registryKey][]string{{usersRoot, alice + `\Volatile Environment`}: {"1"}},
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
		`relocation hint: HKU\S-1-5-21-1-1001\Volatile Environment\AMP_HOME names AMP_HOME ` +
			`(amp) = C:\Users\alice\AppData\Local\amp`,
		`relocation hint: HKU\S-1-5-21-1-1001\Volatile Environment\1\CLINE_DIR names ` +
			`CLINE_DIR (cline) = C:\Users\alice\.codex\cline`,
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
	if first != wantFirst || run.result.Resolved != 8 || len(run.result.Hints) != 9 {
		t.Errorf("first hint %+v, %d resolved of %d hints", first, run.result.Resolved,
			len(run.result.Hints))
	}
	volatile := `HKU\S-1-5-21-1-1001\Volatile Environment\`
	wantVolatile := []Hint{{User: "alice", Source: volatile + "AMP_HOME", Variable: "AMP_HOME",
		Agent: "amp", Value: `C:\Users\alice\AppData\Local\amp`},
		{User: "alice", Source: volatile + `1\CLINE_DIR`, Variable: "CLINE_DIR", Agent: "cline",
			Value: `C:\Users\alice\.codex\cline`}}
	if len(run.result.Hints) == 9 && !slices.Equal(run.result.Hints[6:8], wantVolatile) {
		t.Errorf("volatile hints %+v, want %+v", run.result.Hints[6:8], wantVolatile)
	}
}

func TestRegistryPassLogsAKeyThatPassesTheBudget(t *testing.T) {
	alice, bob := "S-1-5-21-1-1001", "S-1-5-21-1-1002"
	pairs := []string{"BIG", strings.Repeat("b", 32000)}
	for i := range 40 {
		pairs = append(pairs, "R"+strconv.Itoa(i)+"*", "%BIG%")
	}
	pairs = append(pairs, "CODEX_HOME*", `%USERPROFILE%\.codex`)
	keys := map[registryKey]RegistryStrings{
		{localMachine, systemEnvironmentKey}: stringValues("CODEX_HOME*", `%SystemDrive%\codex`),
		{localMachine, windowsVersionKey}:    stringValues("SystemRoot", `C:\Windows`),
		{usersRoot, alice + `\Environment`}:  stringValues(pairs...),
		{usersRoot, bob + `\Environment`}:    stringValues("CODEX_HOME*", `%USERPROFILE%\.codex`),
	}
	profiles := []users.Profile{{SecurityID: alice, User: "alice", Path: `C:\Users\alice`},
		{SecurityID: bob, User: "bob", Path: `C:\Users\bob`}}
	for _, profile := range profiles {
		keys[registryKey{localMachine, profileListKey + profile.SecurityID}] =
			stringValues("ProfileImagePath", profile.Path)
	}
	homes := []users.Home{{User: "alice", Path: `C:\Users\alice`}, {User: "bob", Path: `C:\Users\bob`}}
	log := &lineLog{}
	registry := fakeRegistry{keys: keys, loaded: []string{alice, bob}}
	if err := registrySearch(t, registry, profiles, homes, log).registryPass(); err != nil {
		t.Fatal(err)
	}
	want := []string{"Relocation hints in the registry",
		`relocation hint: HKLM\` + systemEnvironmentKey + `\CODEX_HOME names CODEX_HOME ` +
			`(codex-cli) = C:\codex`,
		`registry: HKU\` + alice + `\Environment: more than 1 MiB of replaced text; the values ` +
			"after that are kept as written",
		`relocation hint: HKU\` + alice + `\Environment\CODEX_HOME names CODEX_HOME ` +
			"(codex-cli); value not resolved: references %USERPROFILE%",
		`relocation hint: HKU\` + bob + `\Environment\CODEX_HOME names CODEX_HOME ` +
			`(codex-cli) = C:\Users\bob\.codex`,
	}
	if !slices.Equal(log.lines, want) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"), strings.Join(want, "\n"))
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

// The keys of alice's hive in the tests of the names a logon gets outside the
// two Environment keys.
const (
	aliceID       = "S-1-5-21-1-1001"
	aliceVolatile = aliceID + `\` + volatileKey
)

// logonRegistry returns a registry holding SystemRoot C:\Windows, the stock
// ProfileList and CurrentVersion values, alice's profile at
// %SystemDrive%\Users\alice with her hive loaded, and then keys, each
// replacing the key of its name, and subkeys as the subkeys of alice's
// Volatile Environment key.
func logonRegistry(keys map[registryKey]RegistryStrings, subkeys []string) fakeRegistry {
	all := map[registryKey]RegistryStrings{
		{localMachine, windowsVersionKey}: stringValues("SystemRoot", `C:\Windows`),
		{localMachine, profilesKey}: stringValues("ProgramData*", `%SystemDrive%\ProgramData`,
			"Public*", `%SystemDrive%\Users\Public`),
		{localMachine, programFilesKey}: stringValues("ProgramFilesDir", `C:\Program Files`,
			"CommonFilesDir", `C:\Program Files\Common Files`,
			"ProgramFilesDir (x86)", `C:\Program Files (x86)`,
			"CommonFilesDir (x86)", `C:\Program Files (x86)\Common Files`,
			"ProgramW6432Dir", `C:\Program Files`, "CommonW6432Dir", `C:\Program Files\Common Files`,
			"ProgramFilesDir (Arm)", `C:\Program Files (Arm)`,
			"CommonFilesDir (Arm)", `C:\Program Files (Arm)\Common Files`),
		{localMachine, profileListKey + aliceID}: stringValues("ProfileImagePath*",
			`%SystemDrive%\Users\alice`),
	}
	maps.Copy(all, keys)
	return fakeRegistry{keys: all, loaded: []string{aliceID},
		subkeys: map[registryKey][]string{{usersRoot, aliceVolatile}: subkeys}}
}

// passLines returns the lines a registry pass over registry logs for alice,
// less the header that comes before the first.
func passLines(t *testing.T, registry fakeRegistry) []string {
	t.Helper()
	log := &lineLog{}
	homes := []users.Home{{User: "alice", Path: `C:\Users\alice`}}
	profiles := []users.Profile{{SecurityID: aliceID, User: "alice", Path: `C:\Users\alice`}}
	if err := registrySearch(t, registry, profiles, homes, log).registryPass(); err != nil {
		t.Fatal(err)
	}
	if len(log.lines) == 0 || log.lines[0] != "Relocation hints in the registry" {
		return log.lines
	}
	return log.lines[1:]
}

// logLine returns the line of a hint of the single-agent variable under
// key (a hint source less the value name): " = value" when result is a
// path, else the reason "references result".
func logLine(key, variable, result string) string {
	agents := map[string]string{"CODEX_HOME": "codex-cli", "CLAUDE_CONFIG_DIR": "claude-code",
		"AMP_HOME": "amp", "CLINE_DIR": "cline", "AUGMENT_VFS_DIR": "augment"}
	line := "relocation hint: " + key + `\` + variable + " names " + variable + " (" +
		agents[variable] + ")"
	if strings.HasPrefix(result, "%") {
		return line + "; value not resolved: references " + result
	}
	return line + " = " + result
}

// The key paths of the hint lines of these tests.
const (
	systemPath   = `HKLM\` + systemEnvironmentKey
	userPath     = `HKU\` + aliceID + `\Environment`
	volatilePath = `HKU\` + aliceVolatile
)

// outsideCase is one case of the names a logon gets outside the two
// Environment keys: the system, user and other keys, the subkeys of the
// Volatile Environment key, and the lines the pass logs.
type outsideCase struct {
	name         string
	system, user RegistryStrings
	keys         map[registryKey]RegistryStrings
	subkeys      []string
	want         []string
}

// startNameCases are the cases of the names set before the system key.
func startNameCases() []outsideCase {
	profiles := registryKey{localMachine, profilesKey}
	return []outsideCase{
		{"ProgramData, ALLUSERSPROFILE and PUBLIC come from ProfileList before the system key",
			stringValues("CLINE_DIR*", `%ProgramData%\s`),
			stringValues("CODEX_HOME*", `%ProgramData%\c`, "CLAUDE_CONFIG_DIR*",
				`%ALLUSERSPROFILE%\a`, "AMP_HOME*", `%PUBLIC%\p`), nil, nil,
			[]string{logLine(systemPath, "CLINE_DIR", `C:\ProgramData\s`),
				logLine(userPath, "CODEX_HOME", `C:\ProgramData\c`),
				logLine(userPath, "CLAUDE_CONFIG_DIR", `C:\ProgramData\a`),
				logLine(userPath, "AMP_HOME", `C:\Users\Public\p`)}},
		{"an expandable source sees only SystemRoot and SystemDrive",
			stringValues("DA_SYS", `C:\dasys`),
			stringValues("CODEX_HOME*", `%ProgramData%\c`, "AMP_HOME*", `%PUBLIC%\p`),
			map[registryKey]RegistryStrings{profiles: stringValues("ProgramData*", `%SystemRoot%\PD`,
				"Public*", `%DA_SYS%\PUB`)}, nil,
			[]string{logLine(userPath, "CODEX_HOME", `C:\Windows\PD\c`),
				logLine(userPath, "AMP_HOME", "%DA_SYS%")}},
		{"a plain source is taken as written", RegistryStrings{},
			stringValues("CODEX_HOME*", `%ProgramData%\c`),
			map[registryKey]RegistryStrings{profiles: stringValues("ProgramData", `%SystemDrive%\PDS`)},
			nil, []string{logLine(userPath, "CODEX_HOME", "%SystemDrive%")}},
		{"an empty or missing source sets neither name", RegistryStrings{},
			stringValues("CODEX_HOME*", `%ProgramData%\c`, "CLAUDE_CONFIG_DIR*",
				`%ALLUSERSPROFILE%\a`, "AMP_HOME*", `%PUBLIC%\p`),
			map[registryKey]RegistryStrings{profiles: stringValues("ProgramData", "")}, nil,
			[]string{logLine(userPath, "CODEX_HOME", "%ProgramData%"),
				logLine(userPath, "CLAUDE_CONFIG_DIR", "%ALLUSERSPROFILE%"),
				logLine(userPath, "AMP_HOME", "%PUBLIC%")}},
		{"a system SystemDrive leaves the profile and ProgramData on SystemRoot's drive",
			stringValues("SystemDrive", "Q:", "CLINE_DIR*", `%SystemDrive%\s`),
			stringValues("CODEX_HOME*", `%ProgramData%\c`, "CLAUDE_CONFIG_DIR*",
				`%USERPROFILE%\a`, "AMP_HOME*", `%SystemDrive%\u`), nil, nil,
			[]string{logLine(systemPath, "CLINE_DIR", `Q:\s`),
				logLine(userPath, "CODEX_HOME", `C:\ProgramData\c`),
				logLine(userPath, "CLAUDE_CONFIG_DIR", `C:\Users\alice\a`),
				logLine(userPath, "AMP_HOME", `Q:\u`)}},
		{"a system value of a start name replaces it and stays, leaving ALLUSERSPROFILE",
			stringValues("ProgramData", `D:\SYSPD`, "PUBLIC", `D:\SYSPUB`),
			stringValues("CODEX_HOME*", `%ProgramData%\c`, "CLAUDE_CONFIG_DIR*",
				`%ALLUSERSPROFILE%\a`, "AMP_HOME*", `%PUBLIC%\p`), nil, nil,
			[]string{logLine(userPath, "CODEX_HOME", `D:\SYSPD\c`),
				logLine(userPath, "CLAUDE_CONFIG_DIR", `C:\ProgramData\a`),
				logLine(userPath, "AMP_HOME", `D:\SYSPUB\p`)}},
	}
}

// programNameCases are the cases of the names set after the system key.
func programNameCases() []outsideCase {
	folders := registryKey{localMachine, programFilesKey}
	return []outsideCase{
		{"the program folders replace a system value for the user, which the system key sees",
			stringValues("ProgramFiles", `D:\SYSPF`, "CLINE_DIR*", `%ProgramFiles%\s`,
				"AUGMENT_VFS_DIR*", `%CommonProgramFiles%\s`),
			stringValues("CODEX_HOME*", `%ProgramFiles%\c`, "CLAUDE_CONFIG_DIR*",
				`%ProgramFiles(x86)%\a`, "AMP_HOME*", `%CommonProgramW6432%\p`), nil, nil,
			[]string{logLine(systemPath, "CLINE_DIR", `D:\SYSPF\s`),
				logLine(systemPath, "AUGMENT_VFS_DIR", "%CommonProgramFiles%"),
				logLine(userPath, "CODEX_HOME", `C:\Program Files\c`),
				logLine(userPath, "CLAUDE_CONFIG_DIR", `C:\Program Files (x86)\a`),
				logLine(userPath, "AMP_HOME", `C:\Program Files\Common Files\p`)}},
		{"an expandable folder is expanded from the drives, and one left with a % is unknown",
			stringValues("DA_SYS", `C:\dasys`),
			stringValues("CODEX_HOME*", `%ProgramFiles%\c`, "CLAUDE_CONFIG_DIR*",
				`%CommonProgramFiles%\a`, "AMP_HOME*", `%ProgramFiles(Arm)%\p`),
			map[registryKey]RegistryStrings{folders: stringValues("ProgramFilesDir*",
				`%SystemDrive%\PFX`, "CommonFilesDir", `%SystemDrive%\CFS`, "ProgramFilesDir (Arm)*",
				`%DA_SYS%\arm`)}, nil,
			[]string{logLine(userPath, "CODEX_HOME", `C:\PFX\c`),
				logLine(userPath, "CLAUDE_CONFIG_DIR", "%CommonProgramFiles%"),
				logLine(userPath, "AMP_HOME", "%ProgramFiles(Arm)%")}},
		{"a missing, empty or repeated folder is unknown, hiding the system value",
			stringValues("ProgramFiles", `D:\SYSPF`, "ProgramW6432", `D:\SYSW`),
			stringValues("CODEX_HOME*", `%ProgramFiles%\c`, "CLAUDE_CONFIG_DIR*",
				`%CommonProgramFiles%\a`, "AMP_HOME*", `%ProgramW6432%\p`),
			map[registryKey]RegistryStrings{folders: stringValues("ProgramFilesDir", "",
				"CommonFilesDir", `C:\a`, "CommonFilesDir", `C:\b`)}, nil,
			[]string{logLine(userPath, "CODEX_HOME", "%ProgramFiles%"),
				logLine(userPath, "CLAUDE_CONFIG_DIR", "%CommonProgramFiles%"),
				logLine(userPath, "AMP_HOME", "%ProgramW6432%")}},
		{"COMPUTERNAME is unknown to the user, though the system key sees its own",
			stringValues("COMPUTERNAME", `C:\cn`, "CLINE_DIR*", `%COMPUTERNAME%\s`),
			stringValues("CODEX_HOME*", `%COMPUTERNAME%\c`), nil, nil,
			[]string{logLine(systemPath, "CLINE_DIR", `C:\cn\s`),
				logLine(userPath, "CODEX_HOME", "%COMPUTERNAME%")}},
		{"a user value sees a user USERNAME, and USERDOMAIN stays", RegistryStrings{},
			stringValues("USERNAME", `C:\un`, "CODEX_HOME*", `%USERNAME%\c`, "AMP_HOME*",
				`%USERDOMAIN%\p`), nil, nil,
			[]string{logLine(userPath, "CODEX_HOME", `C:\un\c`),
				logLine(userPath, "AMP_HOME", "%USERDOMAIN%")}},
	}
}

// volatileCases are the cases of the Volatile Environment key and its
// session subkeys.
func volatileCases() []outsideCase {
	volatile, session := registryKey{usersRoot, aliceVolatile}, registryKey{usersRoot,
		aliceVolatile + `\0`}
	return []outsideCase{
		{"the Volatile Environment key is read after the user key, over it",
			RegistryStrings{}, stringValues("DA_U", `C:\u`, "CLINE_DIR*", `%DA_VOL%\u`),
			map[registryKey]RegistryStrings{volatile: stringValues("CODEX_HOME*", `%DA_VOL%\c`,
				"DA_VOL", `C:\vol`, "CLAUDE_CONFIG_DIR*", `%DA_U%\v`, "AMP_HOME*",
				`%ProgramData%\v`)}, nil,
			[]string{logLine(userPath, "CLINE_DIR", "%DA_VOL%"),
				logLine(volatilePath, "CODEX_HOME", `C:\vol\c`),
				logLine(volatilePath, "CLAUDE_CONFIG_DIR", `C:\u\v`),
				logLine(volatilePath, "AMP_HOME", `C:\ProgramData\v`)}},
		{"a volatile LibPath joins the system's, and a volatile value replaces the profile's",
			stringValues("LibPath", `C:\mlib`),
			RegistryStrings{}, map[registryKey]RegistryStrings{volatile: stringValues("LibPath",
				`V:\vlib`, "USERPROFILE", `D:\vp`, "ProgramFiles", `D:\vpf`, "CODEX_HOME*",
				`%LibPath%\x`, "CLAUDE_CONFIG_DIR*", `%USERPROFILE%\a`, "AMP_HOME*",
				`%ProgramFiles%\p`)}, nil,
			[]string{logLine(volatilePath, "CODEX_HOME", `C:\mlib;V:\vlib\x`),
				logLine(volatilePath, "CLAUDE_CONFIG_DIR", `D:\vp\a`),
				logLine(volatilePath, "AMP_HOME", `D:\vpf\p`)}},
		{"each session subkey is read over the top level alone, and other subkeys are not read",
			RegistryStrings{}, RegistryStrings{},
			map[registryKey]RegistryStrings{volatile: stringValues("DA_TOPV", `C:\top`,
				"CODEX_HOME*", `%DA_SUBV%\t`),
				session: stringValues("DA_SUBV", `C:\sub`, "CLAUDE_CONFIG_DIR*", `%DA_TOPV%\s`,
					"CODEX_HOME", `C:\subcodex`),
				{usersRoot, aliceVolatile + `\x1`}: stringValues("AMP_HOME", `C:\x1`),
				{usersRoot, aliceVolatile + `\1`}:  stringValues("AMP_HOME*", `%DA_SUBV%\one`)},
			[]string{"0", "x1", "1"},
			[]string{logLine(volatilePath, "CODEX_HOME", "%DA_SUBV%"),
				logLine(volatilePath+`\0`, "CLAUDE_CONFIG_DIR", `C:\top\s`),
				logLine(volatilePath+`\0`, "CODEX_HOME", `C:\subcodex`),
				logLine(volatilePath+`\1`, "AMP_HOME", "%DA_SUBV%")}},
		{"a Volatile Environment key that cannot be read is logged, and no subkey is read",
			RegistryStrings{}, stringValues("CODEX_HOME", `C:\u`),
			map[registryKey]RegistryStrings{volatile: {Reason: "access denied"},
				session: stringValues("AMP_HOME", `C:\s`)}, []string{"0"},
			[]string{logLine(userPath, "CODEX_HOME", `C:\u`),
				"registry: " + volatilePath + " not readable: access denied"}},
		{"a session subkey that cannot be read is logged, and the next is read",
			RegistryStrings{}, RegistryStrings{},
			map[registryKey]RegistryStrings{session: {Reason: "access denied"},
				{usersRoot, aliceVolatile + `\1`}: stringValues("AMP_HOME", `C:\one`)},
			[]string{"0", "1"},
			[]string{"registry: " + volatilePath + `\0 not readable: access denied`,
				logLine(volatilePath+`\1`, "AMP_HOME", `C:\one`)}},
	}
}

func TestRegistryPassSetsTheNamesALogonGetsOutsideTheEnvironmentKeys(t *testing.T) {
	t.Parallel()
	tests := slices.Concat(startNameCases(), programNameCases(), volatileCases())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			keys := maps.Clone(test.keys)
			if keys == nil {
				keys = map[registryKey]RegistryStrings{}
			}
			keys[registryKey{localMachine, systemEnvironmentKey}] = test.system
			keys[registryKey{usersRoot, aliceID + `\Environment`}] = test.user
			got := passLines(t, logonRegistry(keys, test.subkeys))
			if !slices.Equal(got, test.want) {
				t.Errorf("log:\n%s\nwant:\n%s", strings.Join(got, "\n"),
					strings.Join(test.want, "\n"))
			}
		})
	}
}
