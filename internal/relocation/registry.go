package relocation

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// RegistryRoot names one of the two registry hives the registry pass reads.
type RegistryRoot int

const (
	// localMachine is HKEY_LOCAL_MACHINE, which holds the system environment.
	localMachine RegistryRoot = iota
	// usersRoot is HKEY_USERS, which holds each loaded user hive under its
	// security identifier.
	usersRoot
)

// The registry keys the registry pass reads: under HKEY_LOCAL_MACHINE the
// system environment, the Windows version key (SystemRoot), ProfileList
// (ProgramData and Public) and each profile's entry under it, and the
// CurrentVersion key (the program folders); under a user hive its folder
// settings and the Volatile Environment key a logon session fills.
const (
	systemEnvironmentKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	windowsVersionKey    = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	profilesKey          = windowsVersionKey + `\ProfileList`
	profileListKey       = profilesKey + `\`
	programFilesKey      = `SOFTWARE\Microsoft\Windows\CurrentVersion`
	shellFoldersKey      = `Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`
	volatileKey          = `Volatile Environment`
)

// expansionLimit is the length past which an expansion is refused and the
// value set as written. Windows has no such limit, but a chain of values that
// each name the one before twice would otherwise grow without bound.
const expansionLimit = 32767

// expansionBudget is how many bytes of replacement text reading one key may
// copy. A replacement that would pass it is not made: that value, and every
// later value of the key that needs a replacement, is set as written, while
// values that need none are unaffected. So reading a key copies at most
// expansionBudget bytes of replacement text, and its memory is bounded by
// the key's own text plus the budget, whatever the shape of the values.
const expansionBudget = 1 << 20

// RegistryStrings is what one registry key holds: every value name in
// registry order, the REG_SZ and REG_EXPAND_SZ values as unexpanded text,
// which of them are REG_EXPAND_SZ, the names whose value could not be read,
// and why the key could not be read ("" when it could). A missing key has
// no names and no reason. Repeated, which readStrings fills, holds each name
// listed more than once, with its ASCII letters lowered.
type RegistryStrings struct {
	Names      []string
	Text       map[string]string
	Expandable map[string]bool
	Unreadable map[string]bool
	Reason     string
	Repeated   map[string]bool
}

// The reasons a registry value has no value, besides acceptRegistryValue's.
const (
	reasonNotAString = "not a string"
	reasonUnreadable = "unreadable"
	reasonRepeated   = "listed more than once"
)

// Registry reads the registry for the registry pass: Strings returns the
// values of subkey under root, and SubKeys the names of its subkeys, nil when
// the key is missing or cannot be listed. The subkeys of HKEY_USERS are the
// security identifiers of the loaded user hives.
type Registry interface {
	Strings(root RegistryRoot, subkey string) RegistryStrings
	SubKeys(root RegistryRoot, subkey string) []string
}

// RegistryOutcome is what the registry pass did: whether it read the
// registry, why not when it did not, and the users whose hive was read, not
// loaded, or loaded but unreadable, in profile order.
type RegistryOutcome struct {
	Read           bool
	Reason         string
	UsersRead      []string
	UsersNotLoaded []string
	UsersDenied    []string
}

// registryPass records the relocation variables of the system environment key
// and of the keys userVariables reads from each collected profile's loaded
// hive, under the registry header; off Windows or in image mode it records
// why it read nothing. A hive that is not loaded is never loaded, since
// loading changes the user's NTUSER.DAT.
func (run *search) registryPass() error {
	run.header = "Relocation hints in the registry"
	outcome := &run.result.Registry
	switch {
	case run.deps.Registry == nil:
		outcome.Reason = "not Windows"
		return nil
	case run.deps.Root != "":
		outcome.Reason = "image mode"
		return nil
	}
	outcome.Read = true
	system := readStrings(run.deps.Registry, localMachine, systemEnvironmentKey)
	if system.Reason != "" {
		err := run.log("registry: system environment not readable: " + system.Reason)
		if err != nil {
			return err
		}
	}
	start, set, overBudget := startLogon(run.deps.Registry, system)
	err := run.registryHints("", `HKLM\`+systemEnvironmentKey, system, set, overBudget)
	if err != nil {
		return err
	}
	return run.userRegistries(start)
}

// logon is what every user's variables are built from: the variables the
// system key left, the names set before it (systemNames) and the names set
// after it (programNames).
type logon struct {
	system          *environment
	names, programs map[string]string
}

// startLogon returns the logon the system key's values give, with the text
// reading them set per name and whether the expansion budget refused one.
func startLogon(registry Registry, system RegistryStrings) (logon, map[string]string, bool) {
	names := systemNames(registry)
	variables := newEnvironment(names)
	set, overBudget := variables.readKey(system, false)
	return logon{system: variables, names: names, programs: programNames(registry, names)},
		set, overBudget
}

// userRegistries reads the keys of each account profile (isAccountIdentifier)
// whose path is a collected home, from start, or lists the profile when its
// hive is not loaded.
func (run *search) userRegistries(start logon) error {
	loaded := run.deps.Registry.SubKeys(usersRoot, "")
	outcome := &run.result.Registry
	for _, profile := range run.deps.Profiles {
		if !isAccountIdentifier(profile.SecurityID) || !run.collectedHome(profile.Path) {
			continue
		}
		hiveLoaded := slices.ContainsFunc(loaded, func(name string) bool {
			return textlines.EqualFoldASCII(name, profile.SecurityID)
		})
		if !hiveLoaded {
			outcome.UsersNotLoaded = append(outcome.UsersNotLoaded, profile.User)
			if err := run.log("registry: hive of " + profile.User + " not loaded"); err != nil {
				return err
			}
			continue
		}
		if err := run.userEnvironment(profile, start); err != nil {
			return err
		}
	}
	return nil
}

// userEnvironment records the hints of each key userVariables reads for the
// profile as it is read, or a line for one that cannot be read, since its
// hints are then missing; when the Environment key cannot be read it lists
// the user as denied instead.
func (run *search) userEnvironment(profile users.Profile, start logon) error {
	outcome := &run.result.Registry
	first := true
	return userVariables(run.deps.Registry, start, profile.SecurityID, func(key userKey) error {
		reason := key.values.Reason
		switch {
		case first && reason != "":
			outcome.UsersDenied = append(outcome.UsersDenied, profile.User)
			return run.log("registry: hive of " + profile.User + " not readable: " + reason)
		case first:
			outcome.UsersRead = append(outcome.UsersRead, profile.User)
		}
		first = false
		if reason != "" {
			return run.log("registry: " + key.path + " not readable: " + reason)
		}
		return run.registryHints(profile.User, key.path, key.values, key.set, key.overBudget)
	})
}

// userKey is one key of a user hive as userVariables read it: its path as a
// hint's source starts, its values, the variables it was read into, the text
// reading it set per name and whether the expansion budget refused one.
type userKey struct {
	path       string
	values     RegistryStrings
	variables  *environment
	set        map[string]string
	overBudget bool
}

// userVariables reads securityID's keys in the order Windows builds a new
// logon's variables: 1. systemNames; 2. the system key; 3. programNames and
// profileValues; 4. the Environment key; 5. its Volatile Environment key, on
// a layer over 4; 6. each session subkey of that (a name of digits), each on
// its own layer over 5, as which session a later process runs in is unknown;
// 7. USERNAME and USERDOMAIN, set after every key, so no hint sees them and
// they are not modelled. Steps 1 to 3 are start's. It calls visit with each
// key as soon as it is read, so a session's values are dropped before the
// next is read, stops at visit's error, and reads nothing after an
// Environment or Volatile Environment key that cannot be read.
func userVariables(registry Registry, start logon, securityID string,
	visit func(userKey) error) error {
	user := readInto(registry, start.forUser(registry, securityID), securityID+`\Environment`)
	if err := visit(user); err != nil || user.values.Reason != "" {
		return err
	}
	volatilePath := securityID + `\` + volatileKey
	volatile := readInto(registry, user.variables.layer(), volatilePath)
	if err := visit(volatile); err != nil || volatile.values.Reason != "" {
		return err
	}
	for _, session := range registry.SubKeys(usersRoot, volatilePath) {
		if session == "" || strings.Trim(session, "0123456789") != "" {
			continue
		}
		key := readInto(registry, volatile.variables.layer(), volatilePath+`\`+session)
		if err := visit(key); err != nil {
			return err
		}
	}
	return nil
}

// readInto reads subkey of the users root into variables as readKey does,
// joining the lists, unless the key cannot be read.
func readInto(registry Registry, variables *environment, subkey string) userKey {
	values := readStrings(registry, usersRoot, subkey)
	key := userKey{path: `HKU\` + subkey, values: values, variables: variables}
	if values.Reason == "" {
		key.set, key.overBudget = variables.readKey(values, true)
	}
	return key
}

// collectedHome reports whether path is the home of one of the run's users,
// ignoring ASCII case on every platform, since a ProfileList path is a
// Windows path wherever the pass is tested. Both sides were already
// respelled as the disk holds them.
func (run *search) collectedHome(path string) bool {
	if path == "" {
		return false
	}
	return slices.ContainsFunc(run.deps.Homes, func(home users.Home) bool {
		return textlines.EqualFoldASCII(home.Path, path)
	})
}

// registryHints records a hint, line 0, for every value of the key whose name
// is a relocation variable in any ASCII case, in registry order, under the
// variable's table spelling, with the key path and value name as source, and
// for a name listed more than once only at its first listing, since the
// listings are one variable. Its value and reason are registryValue's. When
// overBudget, a line before the hints says the key passed expansionBudget, so
// a responder knows why a later value was left with its references.
func (run *search) registryHints(user, keyPath string, values RegistryStrings,
	set map[string]string, overBudget bool) error {
	if overBudget {
		err := run.log("registry: " + keyPath +
			": more than 1 MiB of replaced text; the values after that are kept as written")
		if err != nil {
			return err
		}
	}
	hinted := map[string]bool{}
	for _, name := range values.Names {
		if hinted[textlines.LowerASCII(name)] {
			continue
		}
		hinted[textlines.LowerASCII(name)] = true
		for _, variable := range run.names {
			if !textlines.EqualFoldASCII(variable, name) {
				continue
			}
			value, reason := registryValue(values, set, name)
			if err := run.record(user, keyPath+`\`+name, 0, variable, value, reason); err != nil {
				return err
			}
		}
	}
	return nil
}

// registryValue returns the hint value of the listed name: the text set holds
// for it (readKey's) as acceptRegistryValue keeps it, else no value and a
// reason saying why: a name listed more than once, an unreadable value or one
// that is not a string.
func registryValue(values RegistryStrings, set map[string]string,
	name string) (value, reason string) {
	switch text, ok := set[name]; {
	case values.Repeated[textlines.LowerASCII(name)]:
		return "", reasonRepeated
	case ok && tooLong(values.Text[name]):
		return "", reasonTooLong
	case ok:
		return acceptRegistryValue(text)
	case values.Unreadable[name]:
		return "", reasonUnreadable
	}
	return "", reasonNotAString
}

// readStrings returns registry's values of subkey under root, with every name
// listed more than once in any ASCII case left without text, as a value that
// cannot be read is, and recorded in Repeated; Text is then a copy, so the
// registry's own is unchanged. The listing cuts a name at its first NUL, so
// two values (one named with a NUL through the native call) can share a
// listed name, and reading by that name gives one value's data for both;
// Windows takes one of them, and which cannot be told from the name.
func readStrings(registry Registry, root RegistryRoot, subkey string) RegistryStrings {
	values := registry.Strings(root, subkey)
	listed := map[string]int{}
	for _, name := range values.Names {
		listed[textlines.LowerASCII(name)]++
	}
	if len(listed) == len(values.Names) {
		return values
	}
	text := map[string]string{}
	values.Repeated = map[string]bool{}
	for _, name := range values.Names {
		if listed[textlines.LowerASCII(name)] > 1 {
			values.Repeated[textlines.LowerASCII(name)] = true
		} else if value, ok := values.Text[name]; ok {
			text[name] = value
		}
	}
	values.Text = text
	return values
}

// environment is the set of variables a new logon has while Windows reads
// the registry, each value under its name with ASCII letters lowered. A
// user's environment is a layer over the system's (below), and each of its
// later keys a layer over the one before: own holds what the layer's names
// and key set, removed the names its key emptied, and unknown the names set
// to a value the program cannot give, so a layer costs only its own values
// and never changes the one below. The system's own holds the system names
// and its key's values.
type environment struct {
	own     map[string]string
	removed map[string]bool
	unknown map[string]bool
	below   *environment
}

// newEnvironment returns the variables names holds, with nothing below.
func newEnvironment(names map[string]string) *environment {
	variables := &environment{own: map[string]string{}}
	for name, value := range names {
		variables.own[textlines.LowerASCII(name)] = value
	}
	return variables
}

// lookup returns the value of name in any ASCII case and whether the program
// knows one: a name removed, never set or set to an unknown value has none,
// so a reference to it is kept as written.
func (variables *environment) lookup(name string) (string, bool) {
	value, set, unknown := variables.resolve(name)
	return value, set && !unknown
}

// resolve returns the value of name in any ASCII case from the nearest layer
// that sets, removes or forgets it: the value, whether the name is set, and
// whether it is set to a value the program does not know (forget's).
func (variables *environment) resolve(name string) (value string, set, unknown bool) {
	key := textlines.LowerASCII(name)
	for layer := variables; layer != nil; layer = layer.below {
		if value, ok := layer.own[key]; ok {
			return value, true, false
		}
		if layer.unknown[key] {
			return "", true, true
		}
		if layer.removed[key] {
			return "", false, false
		}
	}
	return "", false, false
}

// forget sets name in this layer to a value the program does not know,
// hiding any value below.
func (variables *environment) forget(name string) {
	key := textlines.LowerASCII(name)
	delete(variables.own, key)
	delete(variables.removed, key)
	if variables.unknown == nil {
		variables.unknown = map[string]bool{}
	}
	variables.unknown[key] = true
}

// layer returns an empty layer over variables, so nothing set in it reaches
// them or another layer over them.
func (variables *environment) layer() *environment {
	return &environment{own: map[string]string{}, removed: map[string]bool{}, below: variables}
}

// forUser returns a layer over the variables the system key left, with the
// program names and the profile's names of securityID set as Windows sets
// them before reading the user's key; a program name mapped to "" is
// forgotten, hiding a system value of its name.
func (start logon) forUser(registry Registry, securityID string) *environment {
	variables := start.system.layer()
	for name, value := range start.programs {
		if value == "" {
			variables.forget(name)
		} else {
			variables.own[textlines.LowerASCII(name)] = value
		}
	}
	for name, value := range profileValues(registry, securityID, start.names) {
		variables.own[textlines.LowerASCII(name)] = value
	}
	return variables
}

// readKey sets the key's values as Windows does for a logon: every REG_SZ
// value as written, then every REG_EXPAND_SZ value expanded once from the
// variables set so far, both in key order. A listed name whose value the
// program cannot give (listed more than once, or unreadable) is unknown from
// the start of the key, since Windows may set it to a value not guessed here,
// so a reference to it, in this key or a later one, stays. An empty value
// removes its variable; another type or a name holding `=` changes nothing.
// Past expansionBudget, values needing a replacement are set as written;
// with joinLists, Path, LibPath and Os2LibPath join as set says. It returns
// the text set per name, before any join, and whether the budget refused one.
func (variables *environment) readKey(values RegistryStrings,
	joinLists bool) (map[string]string, bool) {
	for _, name := range values.Names {
		if (values.Repeated[textlines.LowerASCII(name)] || values.Unreadable[name]) &&
			!strings.Contains(name, "=") {
			variables.forget(name)
		}
	}
	written := map[string]string{}
	left, overBudget := expansionBudget, false
	for _, expandable := range []bool{false, true} {
		for _, name := range values.Names {
			text, ok := values.Text[name]
			if !ok || values.Expandable[name] != expandable || strings.Contains(name, "=") {
				continue
			}
			if expandable && !overBudget {
				expanded, cost, refused := expandReferences(text, variables, left)
				text, left, overBudget = expanded, left-cost, refused
			}
			written[name] = text
			variables.set(name, text, joinLists)
		}
	}
	return written, overBudget
}

// set gives name the value text in this layer, or removes it when text is
// empty, hiding a value below. A user's Path, LibPath or Os2LibPath
// (joinLists) is added after the existing value and a `;`, with no `;` when
// that value is empty or already ends with one. When the joined value would
// pass expansionLimit, the variable keeps the existing value, so a large
// system Path is not copied once per user; joined to an unknown value, it is
// unknown too.
func (variables *environment) set(name, text string, joinLists bool) {
	key := textlines.LowerASCII(name)
	delete(variables.unknown, key)
	if text == "" {
		delete(variables.own, key)
		if variables.below != nil {
			variables.removed[key] = true
		}
		return
	}
	existing, _, unknown := variables.resolve(key)
	isList := joinLists && slices.Contains([]string{"path", "libpath", "os2libpath"}, key)
	if isList && unknown {
		variables.forget(key)
		return
	}
	if isList && existing != "" {
		separator := ";"
		if strings.HasSuffix(existing, ";") {
			separator = ""
		}
		if len(existing)+len(separator)+len(text) > expansionLimit {
			return
		}
		text = existing + separator + text
	}
	variables.own[key] = text
}

// expandReferences replaces each %NAME% reference of text in one left-to-right
// scan, never scanning a replacement again. A name is the text up to the next
// `%`; when no variable has it, the opening `%` and the name are kept and the
// scan goes on at the closing `%`, which may open the next reference. Before
// each replacement is copied it checks that the result stays within
// expansionLimit and the copy within allowance, the bytes of replacement text
// it may copy; when either fails, text is returned as written. It also
// returns the bytes of replacement text it copied, refused or not, and
// whether the allowance (not the limit) refused it.
func expandReferences(text string, variables *environment,
	allowance int) (result string, cost int, overAllowance bool) {
	var expanded strings.Builder
	position := 0
	for position < len(text) {
		start := strings.IndexByte(text[position:], '%')
		if start < 0 {
			expanded.WriteString(text[position:])
			break
		}
		start += position
		expanded.WriteString(text[position:start])
		length := strings.IndexByte(text[start+1:], '%')
		if length < 0 {
			expanded.WriteString(text[start:])
			break
		}
		end := start + 1 + length
		value, found := variables.lookup(text[start+1 : end])
		if !found || length == 0 {
			expanded.WriteString(text[start:end])
			position = end
			continue
		}
		if expanded.Len()+len(value) > expansionLimit {
			return text, cost, false
		}
		if cost+len(value) > allowance {
			return text, cost, true
		}
		expanded.WriteString(value)
		cost += len(value)
		position = end + 1
	}
	if expanded.Len() > expansionLimit {
		return text, cost, false
	}
	return expanded.String(), cost, false
}

// lookupFold returns name's value in table in any ASCII case. The registry
// also folds case outside ASCII, so a name differing by a non-ASCII pair
// (U+017F for s) matches there and not here, on purpose. A registry key holds
// no two value names differing only by case, so at most one key matches.
func lookupFold(table map[string]string, name string) (string, bool) {
	if value, ok := table[name]; ok {
		return value, true
	}
	for key, value := range table {
		if textlines.EqualFoldASCII(key, name) {
			return value, true
		}
	}
	return "", false
}

// systemNames returns the names a logon has before the system key, as far as
// known: SystemRoot from the Windows version key and SystemDrive, its drive,
// which a system key value of that name does not change for the profile path
// or these names; ProgramData and ALLUSERSPROFILE from ProfileList's
// ProgramData value, and PUBLIC from its Public value (sourceValue's, from
// SystemRoot and SystemDrive); a missing value sets nothing.
func systemNames(registry Registry) map[string]string {
	names := map[string]string{}
	version := readStrings(registry, localMachine, windowsVersionKey)
	if root, ok := lookupFold(version.Text, "SystemRoot"); ok {
		names["SystemRoot"] = root
		if len(root) >= 2 && isLetter(root[0]) && root[1] == ':' {
			names["SystemDrive"] = root[:2]
		}
	}
	start := driveNames(names)
	profiles := readStrings(registry, localMachine, profilesKey)
	if value, ok := sourceValue(profiles, "ProgramData", start); ok {
		names["ProgramData"], names["ALLUSERSPROFILE"] = value, value
	}
	if value, ok := sourceValue(profiles, "Public", start); ok {
		names["PUBLIC"] = value
	}
	return names
}

// programNames returns the names Windows sets after the system key and before
// the user's, each under its variable: the program folders from the
// CurrentVersion key (sourceValue's, from SystemRoot and SystemDrive), and
// COMPUTERNAME, which no registry value gives. A name whose value the program
// cannot give (a source missing, listed twice or unreadable, or a value left
// holding a %) maps to "", as COMPUTERNAME always does.
func programNames(registry Registry, names map[string]string) map[string]string {
	sources := [][2]string{{"ProgramFiles", "ProgramFilesDir"},
		{"CommonProgramFiles", "CommonFilesDir"}, {"ProgramFiles(x86)", "ProgramFilesDir (x86)"},
		{"CommonProgramFiles(x86)", "CommonFilesDir (x86)"}, {"ProgramW6432", "ProgramW6432Dir"},
		{"CommonProgramW6432", "CommonW6432Dir"}, {"ProgramFiles(Arm)", "ProgramFilesDir (Arm)"},
		{"CommonProgramFiles(Arm)", "CommonFilesDir (Arm)"}}
	folders := readStrings(registry, localMachine, programFilesKey)
	start := driveNames(names)
	programs := map[string]string{"COMPUTERNAME": ""}
	for _, source := range sources {
		value, ok := sourceValue(folders, source[1], start)
		if !ok || strings.Contains(value, "%") {
			value = ""
		}
		programs[source[0]] = value
	}
	return programs
}

// driveNames returns the variables holding only the SystemRoot and SystemDrive
// of names, the ones Windows expands the values it reads outside the
// Environment keys from.
func driveNames(names map[string]string) *environment {
	drives := map[string]string{}
	for _, name := range []string{"SystemRoot", "SystemDrive"} {
		if value, ok := names[name]; ok {
			drives[name] = value
		}
	}
	return newEnvironment(drives)
}

// sourceValue returns the value a logon takes from the value named source in
// values, in any ASCII case: a REG_SZ value as written, a REG_EXPAND_SZ one
// expanded once from start. ok is false when the value is missing, empty, of
// another type, listed more than once or unreadable, as Windows then sets
// nothing from it or a value the program cannot pick out.
func sourceValue(values RegistryStrings, source string, start *environment) (string, bool) {
	for _, name := range values.Names {
		text, ok := values.Text[name]
		if !ok || text == "" || !textlines.EqualFoldASCII(name, source) {
			continue
		}
		if values.Expandable[name] {
			text, _, _ = expandReferences(text, start, expansionLimit)
		}
		return text, true
	}
	return "", false
}

// profileValues returns the profile variables Windows sets before reading a
// user's key: USERPROFILE (the ProfileList path expanded once with SystemRoot
// and SystemDrive only), and APPDATA and LOCALAPPDATA from the hive's User
// Shell Folders key, expanded once with USERPROFILE and those two, else under
// AppData. It is empty when the profile path does not expand to a drive path.
func profileValues(registry Registry, securityID string,
	names map[string]string) map[string]string {
	values := map[string]string{}
	entry := readStrings(registry, localMachine, profileListKey+securityID)
	raw, ok := lookupFold(entry.Text, "ProfileImagePath")
	if !ok {
		return values
	}
	known := driveNames(names)
	expandedProfile, _, _ := expandReferences(raw, known, expansionLimit)
	profile := strings.TrimRight(expandedProfile, `\`)
	if !isDrivePath(profile, `\`) || strings.Contains(profile, "%") {
		return values
	}
	values["USERPROFILE"] = profile
	values["APPDATA"] = profile + `\AppData\Roaming`
	values["LOCALAPPDATA"] = profile + `\AppData\Local`
	folders := readStrings(registry, usersRoot, securityID+`\`+shellFoldersKey)
	known.set("USERPROFILE", profile, false)
	if roaming, ok := lookupFold(folders.Text, "AppData"); ok {
		values["APPDATA"], _, _ = expandReferences(roaming, known, expansionLimit)
	}
	if local, ok := lookupFold(folders.Text, "Local AppData"); ok {
		values["LOCALAPPDATA"], _, _ = expandReferences(local, known, expansionLimit)
	}
	return values
}

// acceptRegistryValue keeps a drive path (X:\ or X:/) of at most
// maxValueBytes holding no control character, %, quote or .. segment, less
// its trailing separators (a bare drive keeps one `\`). Otherwise value is
// empty and reason names the first problem, a length first and then in that
// order, naming as %NAME% the text between the first `%` and the next, which
// Windows would have read as a name. A `$` or a backquote is an ordinary
// Windows path character the registry does not expand.
func acceptRegistryValue(text string) (value, reason string) {
	_, afterPercent, percent := strings.Cut(text, "%")
	switch {
	case tooLong(text):
		return "", reasonTooLong
	case hasControl(text):
		return "", "holds a control character"
	case percent:
		if name, _, closed := strings.Cut(afterPercent, "%"); closed && name != "" {
			return "", "references %" + name + "%"
		}
		return "", "holds a %"
	case strings.Contains(text, `"`):
		return "", "holds a quote"
	case !isDrivePath(text, `\/`):
		return "", "not a drive path"
	case slices.Contains(strings.Split(strings.ReplaceAll(text, "/", `\`), `\`), ".."):
		return "", "holds a .. segment"
	}
	value = strings.TrimRight(text, `\/`)
	if len(value) == 2 {
		value += `\`
	}
	return value, ""
}

// isDrivePath reports whether text starts with an ASCII drive letter, a colon
// and one of separators.
func isDrivePath(text, separators string) bool {
	return len(text) >= 3 && isLetter(text[0]) && text[1] == ':' &&
		strings.IndexByte(separators, text[2]) >= 0
}

// isAccountIdentifier reports a security identifier of a local or domain
// account (S-1-5-21-...) or a Microsoft Entra account (S-1-12-...), in any
// ASCII case; the registry pass reads no other profile (S-1-5-18 and the
// other service accounts).
func isAccountIdentifier(securityID string) bool {
	for _, prefix := range []string{"s-1-5-21-", "s-1-12-"} {
		if len(securityID) >= len(prefix) && textlines.EqualFoldASCII(securityID[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}
