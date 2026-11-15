package relocation

import (
	"slices"
	"strings"
)

// RegistryRoot names one of the two registry hives the registry pass reads.
type RegistryRoot int

const (
	// LocalMachine is HKEY_LOCAL_MACHINE, which holds the system environment.
	LocalMachine RegistryRoot = iota
	// UsersRoot is HKEY_USERS, which holds each loaded user hive under its
	// security identifier.
	UsersRoot
)

// The registry keys the registry pass reads: the system environment, the
// Windows version key (SystemRoot), each profile's ProfileList entry under
// HKEY_LOCAL_MACHINE, and a user hive's folder settings under HKEY_USERS.
const (
	systemEnvironmentKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	windowsVersionKey    = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	profileListKey       = windowsVersionKey + `\ProfileList\`
	shellFoldersKey      = `Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`
)

// expansionPasses and expansionLimit bound how often a value's %NAME%
// references are replaced and the text length past which no pass starts, so a
// value that refers to itself ends with the reference left in.
const (
	expansionPasses = 8
	expansionLimit  = 32767
)

// RegistryStrings is what one registry key holds: every value name in
// registry order, the REG_SZ and REG_EXPAND_SZ values as name to raw text,
// never expanded, which of those are REG_EXPAND_SZ, and why the key could not
// be read ("" when it could). A missing key has no names and no reason.
type RegistryStrings struct {
	Names      []string
	Text       map[string]string
	Expandable map[string]bool
	Reason     string
}

// Registry reads the registry for the registry pass: Strings returns the
// values of subkey under root, and LoadedHives the key names under
// HKEY_USERS, the security identifiers of the loaded user hives.
type Registry interface {
	Strings(root RegistryRoot, subkey string) RegistryStrings
	LoadedHives() []string
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

// Profile is one ProfileList entry: the key's security identifier, the user
// (the profile path's last component) and the profile path in canonical form
// without a trailing separator.
type Profile struct {
	SecurityID string
	User       string
	Path       string
}

// registryPass records the relocation variables of the system environment key
// and of the Environment key of each collected profile's loaded hive, under
// the registry header; off Windows or in image mode it records why it read
// nothing. A hive that is not loaded is never loaded, since loading changes
// the user's NTUSER.DAT.
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
	system := run.deps.Registry.Strings(LocalMachine, systemEnvironmentKey)
	if system.Reason != "" {
		err := run.log("registry: system environment not readable: " + system.Reason)
		if err != nil {
			return err
		}
	}
	systemTables := []map[string]string{system.Text, systemNames(run.deps.Registry, system.Text)}
	err := run.registryHints("", `HKLM\`+systemEnvironmentKey, system, systemTables)
	if err != nil {
		return err
	}
	return run.userRegistries(systemTables)
}

// userRegistries reads the Environment key of each profile whose security
// identifier is a local, domain or Microsoft Entra account's
// (S-1-5-21-... or S-1-12-...) and whose path is a collected home, listing a
// profile whose hive is not loaded instead.
func (run *search) userRegistries(systemTables []map[string]string) error {
	loaded := run.deps.Registry.LoadedHives()
	outcome := &run.result.Registry
	for _, profile := range run.deps.Profiles {
		if !isAccountIdentifier(profile.SecurityID) || !run.collectedHome(profile.Path) {
			continue
		}
		hiveLoaded := slices.ContainsFunc(loaded, func(name string) bool {
			return equalFoldASCII(name, profile.SecurityID)
		})
		if !hiveLoaded {
			outcome.UsersNotLoaded = append(outcome.UsersNotLoaded, profile.User)
			if err := run.log("registry: hive of " + profile.User + " not loaded"); err != nil {
				return err
			}
			continue
		}
		if err := run.userEnvironment(profile, systemTables); err != nil {
			return err
		}
	}
	return nil
}

// userEnvironment records the hints of a profile's Environment key, expanding
// its values from the key itself, the profile's values and the system tables,
// or lists the user as denied when the key cannot be read.
func (run *search) userEnvironment(profile Profile, systemTables []map[string]string) error {
	outcome := &run.result.Registry
	environment := run.deps.Registry.Strings(UsersRoot, profile.SecurityID+`\Environment`)
	if environment.Reason != "" {
		outcome.UsersDenied = append(outcome.UsersDenied, profile.User)
		return run.log("registry: hive of " + profile.User + " not readable: " +
			environment.Reason)
	}
	outcome.UsersRead = append(outcome.UsersRead, profile.User)
	profileTable := profileValues(run.deps.Registry, profile.SecurityID, systemTables[1])
	tables := append([]map[string]string{environment.Text, profileTable}, systemTables...)
	keyPath := `HKU\` + profile.SecurityID + `\Environment`
	return run.registryHints(profile.User, keyPath, environment, tables)
}

// collectedHome reports whether path is the home of one of the run's users,
// ignoring ASCII case on every platform, since a ProfileList path is a
// Windows path wherever the pass is tested. Both sides were respelled as the
// disk holds them when the users were found, so no other case is folded.
func (run *search) collectedHome(path string) bool {
	if path == "" {
		return false
	}
	for _, home := range run.deps.Homes {
		if equalFoldASCII(home.Path, path) {
			return true
		}
	}
	return false
}

// registryHints records a hint, line 0, for every value of the key whose name
// is a relocation variable in any ASCII case, in registry order, under the variable's
// table spelling with the key path and the value's own name as source. A
// REG_EXPAND_SZ value is expanded from tables first; a value that is not a
// string has no value.
func (run *search) registryHints(user, keyPath string, values RegistryStrings,
	tables []map[string]string) error {
	for _, name := range values.Names {
		for _, variable := range run.names {
			if !equalFoldASCII(variable, name) {
				continue
			}
			value, reason := "", "not a string"
			if text, ok := values.Text[name]; ok {
				if values.Expandable[name] {
					text = expandRegistryValue(text, tables)
				}
				value, reason = acceptRegistryValue(text)
			}
			if err := run.record(user, keyPath+`\`+name, 0, variable, value, reason); err != nil {
				return err
			}
		}
	}
	return nil
}

// expandRegistryValue replaces each %NAME% reference of text from the first
// table that holds the name in any case, then does so again on the result, up
// to eight passes and while the text is at most 32767 bytes long; a pass that
// replaces nothing ends it. A reference no table holds stays as written.
func expandRegistryValue(text string, tables []map[string]string) string {
	for pass := 0; pass < expansionPasses && len(text) <= expansionLimit; pass++ {
		expanded, changed := expandOnce(text, tables)
		if !changed {
			break
		}
		text = expanded
	}
	return text
}

// expandOnce replaces each %NAME% reference of text, scanned left to right
// without overlap, with its value from tables, and reports whether any was.
func expandOnce(text string, tables []map[string]string) (string, bool) {
	var expanded strings.Builder
	changed, last := false, 0
	for start := strings.IndexByte(text, '%'); start >= 0; {
		name, ok := referenceAt(text, start)
		if !ok {
			start = nextPercent(text, start+1)
			continue
		}
		end := start + len(name) + 2
		expanded.WriteString(text[last:start])
		if replacement, found := lookupTables(tables, name); found {
			expanded.WriteString(replacement)
			changed = true
		} else {
			expanded.WriteString(text[start:end])
		}
		last, start = end, nextPercent(text, end)
	}
	expanded.WriteString(text[last:])
	return expanded.String(), changed
}

// nextPercent returns the index of the first `%` of text at or after from, or
// -1.
func nextPercent(text string, from int) int {
	if from >= len(text) {
		return -1
	}
	if offset := strings.IndexByte(text[from:], '%'); offset >= 0 {
		return from + offset
	}
	return -1
}

// referenceAt returns the NAME of the %NAME% reference starting at the `%` at
// index start: a letter or underscore, then letters, digits, underscores and
// parentheses (ProgramFiles(x86)), then a closing `%`.
func referenceAt(text string, start int) (string, bool) {
	after := text[start+1:]
	if after == "" || !isNameStart(after[0]) {
		return "", false
	}
	n := 1
	for n < len(after) && (isWordByte(after[n]) || after[n] == '(' || after[n] == ')') {
		n++
	}
	if n == len(after) || after[n] != '%' {
		return "", false
	}
	return after[:n], true
}

// lookupTables returns name's value from the first table holding it in any
// case.
func lookupTables(tables []map[string]string, name string) (string, bool) {
	for _, table := range tables {
		if value, ok := lookupFold(table, name); ok {
			return value, true
		}
	}
	return "", false
}

// lookupFold returns name's value in table in any ASCII case. The registry
// and a PowerShell hashtable also ignore case outside ASCII, so a name
// differing by a non-ASCII case pair (U+017F for s) matches there and not
// here, a decided difference. A registry key holds no two value names
// differing only by case, so at most one key matches.
func lookupFold(table map[string]string, name string) (string, bool) {
	if value, ok := table[name]; ok {
		return value, true
	}
	for key, value := range table {
		if equalFoldASCII(key, name) {
			return value, true
		}
	}
	return "", false
}

// systemNames returns the names a profile path is written with, as far as
// known: SystemRoot from the Windows version key, and SystemDrive from the
// system environment, else SystemRoot's drive. Neither is in the system key
// on a usual host.
func systemNames(registry Registry, systemText map[string]string) map[string]string {
	names := map[string]string{}
	version := registry.Strings(LocalMachine, windowsVersionKey)
	if root, ok := lookupFold(version.Text, "SystemRoot"); ok {
		names["SystemRoot"] = root
	}
	if drive, ok := lookupFold(systemText, "SystemDrive"); ok {
		names["SystemDrive"] = drive
	} else if root := names["SystemRoot"]; len(root) >= 2 && isLetter(root[0]) && root[1] == ':' {
		names["SystemDrive"] = root[:2]
	}
	return names
}

// profileValues returns the profile variables a user value may refer to:
// USERPROFILE (the ProfileList path expanded with the system names only),
// HOMEDRIVE and HOMEPATH split from it, and APPDATA and LOCALAPPDATA from the
// hive's User Shell Folders key, else under AppData. It is empty when the
// profile path does not expand to a drive path.
func profileValues(registry Registry, securityID string,
	names map[string]string) map[string]string {
	values := map[string]string{}
	entry := registry.Strings(LocalMachine, profileListKey+securityID)
	raw, ok := lookupFold(entry.Text, "ProfileImagePath")
	if !ok {
		return values
	}
	profile := strings.TrimRight(expandRegistryValue(raw, []map[string]string{names}), `\`)
	if !isDrivePath(profile, `\`) || strings.Contains(profile, "%") {
		return values
	}
	values["USERPROFILE"] = profile
	values["HOMEDRIVE"] = profile[:2]
	values["HOMEPATH"] = profile[2:]
	values["APPDATA"] = profile + `\AppData\Roaming`
	values["LOCALAPPDATA"] = profile + `\AppData\Local`
	folders := registry.Strings(UsersRoot, securityID+`\`+shellFoldersKey)
	if roaming, ok := lookupFold(folders.Text, "AppData"); ok {
		values["APPDATA"] = roaming
	}
	if local, ok := lookupFold(folders.Text, "Local AppData"); ok {
		values["LOCALAPPDATA"] = local
	}
	return values
}

// acceptRegistryValue keeps a registry value that is a drive path (X:\ or
// X:/) holding no control character, no %, no quote and no .. segment, without
// its trailing separators (a bare drive keeps one `\`). Otherwise value is
// empty and reason names what was found, in that order, a % that starts a
// %NAME% reference left unreplaced naming it. A `$` or a backquote is an
// ordinary path character on Windows, and the registry expands neither.
func acceptRegistryValue(text string) (value, reason string) {
	percent := strings.IndexByte(text, '%')
	switch {
	case hasControlOrNUL(text):
		return "", "holds a control character"
	case percent >= 0:
		if name, ok := referenceAt(text, percent); ok {
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

// hasControlOrNUL reports a byte from 0x00 to 0x1f, or 0x7f, in text.
func hasControlOrNUL(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < 0x20 || text[i] == 0x7f {
			return true
		}
	}
	return false
}

// isDrivePath reports whether text starts with an ASCII drive letter, a colon
// and one of separators.
func isDrivePath(text, separators string) bool {
	return len(text) >= 3 && isLetter(text[0]) && text[1] == ':' &&
		strings.IndexByte(separators, text[2]) >= 0
}

// isAccountIdentifier reports a security identifier of a local or domain
// account (S-1-5-21-...) or a Microsoft Entra account (S-1-12-...), in any
// ASCII case as PowerShell's -match compares; the registry pass reads no other
// profile (S-1-5-18 and the other service accounts).
func isAccountIdentifier(securityID string) bool {
	for _, prefix := range []string{"s-1-5-21-", "s-1-12-"} {
		if len(securityID) >= len(prefix) && equalFoldASCII(securityID[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}

// equalFoldASCII reports whether first and second are equal when ASCII letters are
// compared without case, the folding the binary applies to registry names,
// security identifiers and Windows paths.
func equalFoldASCII(first, second string) bool {
	return len(first) == len(second) && lowerASCII(first) == lowerASCII(second)
}
