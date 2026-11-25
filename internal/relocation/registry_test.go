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

func TestReadKeySetsAListedTwiceNameOnce(t *testing.T) {
	t.Parallel()
	values := stringValues("CODEX_HOME*", `%CODEX_HOME%\x`)
	values.Names = append(values.Names, "CODEX_HOME")
	variables := newEnvironment(map[string]string{"CODEX_HOME": `C:\sys`})
	set, _ := variables.readKey(values, true)
	if got := set["CODEX_HOME"]; got != `C:\sys\x` {
		t.Errorf("CODEX_HOME = %q, want C:\\sys\\x", got)
	}
	if got, _ := variables.lookup("CODEX_HOME"); got != `C:\sys\x` {
		t.Errorf("variable = %q, want C:\\sys\\x", got)
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
	system := newEnvironment(names)
	user := stringValues("CODEX_HOME*", `%SYSTEM_VARIABLE_7%\codex`)
	const users, perUser = 200, 4096
	allocated := allocatedBy(func() {
		for i := range users {
			system.forUser(fakeRegistry{}, "S-1-5-21-1-"+strconv.Itoa(i), nil).readKey(user, true)
		}
	})
	if bound := uint64(4 * (systemSize + users*perUser)); allocated > bound {
		t.Errorf("%d users allocated %d bytes, want at most %d", users, allocated, bound)
	}
}

func TestAUserKeyChangesOnlyThatUsersVariables(t *testing.T) {
	t.Parallel()
	system := newEnvironment(map[string]string{"DA_Z1": `C:\z1`, "DA_K1": `C:\sys`})
	first := system.forUser(fakeRegistry{}, "S-1-5-21-1-1001", nil)
	set, _ := first.readKey(stringValues("DA_Z1", "", "da_k1", `C:\user`, "DA_R1*", `%DA_Z1%\x`,
		"DA_R2*", `%Da_K1%\x`), true)
	second := system.forUser(fakeRegistry{}, "S-1-5-21-1-1002", nil)
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
	first := system.forUser(registry, "S-1-5-21-1-1001", nil)
	first.readKey(stringValues("ONLYFIRST", `C:\first`), true)
	second := system.forUser(registry, "S-1-5-21-1-1002", nil)
	set, _ := second.readKey(stringValues("OTHER*", `%ONLYFIRST%\x`), true)
	if got := set["OTHER"]; got != `%ONLYFIRST%\x` {
		t.Errorf("OTHER = %q, want the reference kept", got)
	}
	if _, leaked := system.lookup("ONLYFIRST"); leaked {
		t.Errorf("the system environment gained ONLYFIRST")
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
