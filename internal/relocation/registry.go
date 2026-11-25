package relocation

import (
	"slices"
	"strings"

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

// The registry keys the registry pass reads: the system environment, the
// Windows version key (SystemRoot), each profile's ProfileList entry under
// HKEY_LOCAL_MACHINE, and a user hive's folder settings under HKEY_USERS.
const (
	systemEnvironmentKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	windowsVersionKey    = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	profileListKey       = windowsVersionKey + `\ProfileList\`
	shellFoldersKey      = `Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`
)

// expansionLimit is the length past which an expansion is refused and the
// value set as written. Windows has no such limit, but a chain of values that
// each name the one before twice would otherwise grow without bound.
const expansionLimit = 32767

// expansionBudget is the total length of the expanded values of one key. Once
// the expansions of a key would pass it, that value and every later
// REG_EXPAND_SZ value of the key are set as written, so many values naming
// one large value cannot cost more than the budget in memory.
const expansionBudget = 1 << 20

// RegistryStrings is what one registry key holds: every value name in
// registry order, the REG_SZ and REG_EXPAND_SZ values as unexpanded text,
// which of them are REG_EXPAND_SZ, the names whose value could not be read,
// and why the key could not be read ("" when it could). A missing key has
// no names and no reason.
type RegistryStrings struct {
	Names      []string
	Text       map[string]string
	Expandable map[string]bool
	Unreadable map[string]bool
	Reason     string
}

// The reasons a registry value has no value, besides acceptRegistryValue's.
const (
	reasonNotAString = "not a string"
	reasonUnreadable = "unreadable"
)

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
	system := run.deps.Registry.Strings(localMachine, systemEnvironmentKey)
	if system.Reason != "" {
		err := run.log("registry: system environment not readable: " + system.Reason)
		if err != nil {
			return err
		}
	}
	names := systemNames(run.deps.Registry, system.Text)
	variables := newEnvironment(names)
	set := variables.readKey(system, false)
	if err := run.registryHints("", `HKLM\`+systemEnvironmentKey, system, set); err != nil {
		return err
	}
	return run.userRegistries(variables, names)
}

// userRegistries reads the Environment key of each account profile
// (isAccountIdentifier) whose path is a collected home, from the variables
// the system key left and the system names, or lists the profile when its
// hive is not loaded.
func (run *search) userRegistries(system environment, names map[string]string) error {
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
		if err := run.userEnvironment(profile, system, names); err != nil {
			return err
		}
	}
	return nil
}

// userEnvironment records the hints of a profile's Environment key, read on
// the variables forUser gives, or lists the user as denied when the key
// cannot be read.
func (run *search) userEnvironment(profile users.Profile, system environment,
	names map[string]string) error {
	outcome := &run.result.Registry
	userKey := run.deps.Registry.Strings(usersRoot, profile.SecurityID+`\Environment`)
	if userKey.Reason != "" {
		outcome.UsersDenied = append(outcome.UsersDenied, profile.User)
		return run.log("registry: hive of " + profile.User + " not readable: " +
			userKey.Reason)
	}
	outcome.UsersRead = append(outcome.UsersRead, profile.User)
	set := system.forUser(run.deps.Registry, profile.SecurityID, names).readKey(userKey, true)
	keyPath := `HKU\` + profile.SecurityID + `\Environment`
	return run.registryHints(profile.User, keyPath, userKey, set)
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
		return equalFoldASCII(home.Path, path)
	})
}

// registryHints records a hint, line 0, for every value of the key whose name
// is a relocation variable in any ASCII case, in registry order, under the
// variable's table spelling, with the key path and value name as source. Its
// value is the text set holds for the name (readKey's); an unreadable or
// non-string value has no value and a reason saying which.
func (run *search) registryHints(user, keyPath string, values RegistryStrings,
	set map[string]string) error {
	for _, name := range values.Names {
		for _, variable := range run.names {
			if !equalFoldASCII(variable, name) {
				continue
			}
			value, reason := "", reasonNotAString
			if values.Unreadable[name] {
				reason = reasonUnreadable
			}
			if text, ok := set[name]; ok {
				value, reason = acceptRegistryValue(text)
			}
			if err := run.record(user, keyPath+`\`+name, 0, variable, value, reason); err != nil {
				return err
			}
		}
	}
	return nil
}

// environment is the set of variables a new logon has while Windows reads
// the registry, each value under its name with ASCII letters lowered.
type environment map[string]string

// newEnvironment returns the variables names holds.
func newEnvironment(names map[string]string) environment {
	variables := environment{}
	for name, value := range names {
		variables[lowerASCII(name)] = value
	}
	return variables
}

// forUser returns a copy of the variables the system key left, so one user's
// values never reach another's, with the profile's names of securityID set
// as Windows sets them before reading the user's key.
func (system environment) forUser(registry Registry, securityID string,
	names map[string]string) environment {
	variables := environment{}
	for name, value := range system {
		variables[name] = value
	}
	for name, value := range profileValues(registry, securityID, names) {
		variables[lowerASCII(name)] = value
	}
	return variables
}

// readKey sets the key's values as Windows does for a logon: every REG_SZ
// value as written, then every REG_EXPAND_SZ value expanded once from the
// variables set so far, both in key order. An empty value removes its
// variable; a value of another type, an unreadable one, a name holding `=` or
// a name already set in this call changes nothing. Once the expansions total
// more than expansionBudget, the rest are set as written. With joinLists, a
// Path, LibPath or Os2LibPath value is added after the existing one. It
// returns the text set under each value name, before any joining.
func (variables environment) readKey(values RegistryStrings, joinLists bool) map[string]string {
	written := map[string]string{}
	spent := 0
	for _, expandable := range []bool{false, true} {
		for _, name := range values.Names {
			text, ok := values.Text[name]
			if _, done := written[name]; done || !ok || values.Expandable[name] != expandable ||
				strings.Contains(name, "=") {
				continue
			}
			if expandable && spent <= expansionBudget {
				expanded := expandReferences(text, variables)
				spent += len(expanded)
				if spent <= expansionBudget {
					text = expanded
				}
			}
			written[name] = text
			variables.set(name, text, joinLists)
		}
	}
	return written
}

// set gives name the value text, or removes it when text is empty. A user's
// Path, LibPath or Os2LibPath (joinLists) is added after the existing value
// and a `;`, with no `;` when that value is empty or already ends with one.
func (variables environment) set(name, text string, joinLists bool) {
	key := lowerASCII(name)
	if text == "" {
		delete(variables, key)
		return
	}
	existing := variables[key]
	if joinLists && slices.Contains([]string{"path", "libpath", "os2libpath"}, key) &&
		existing != "" {
		if !strings.HasSuffix(existing, ";") {
			existing += ";"
		}
		text = existing + text
	}
	variables[key] = text
}

// expandReferences replaces each %NAME% reference of text in one left-to-right
// scan, never scanning a replacement again. A name is the text up to the next
// `%`; when no variable has it, the opening `%` and the name are kept and the
// scan goes on at the closing `%`, which may open the next reference. When the
// result would pass expansionLimit, text is returned as written, checked
// before each replacement is copied so the cost stays linear in text.
func expandReferences(text string, variables environment) string {
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
		value, found := variables[lowerASCII(text[start+1:end])]
		if !found || length == 0 {
			expanded.WriteString(text[start:end])
			position = end
			continue
		}
		if expanded.Len()+len(value) > expansionLimit {
			return text
		}
		expanded.WriteString(value)
		position = end + 1
	}
	if expanded.Len() > expansionLimit {
		return text
	}
	return expanded.String()
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
	version := registry.Strings(localMachine, windowsVersionKey)
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

// profileValues returns the profile variables Windows sets before reading a
// user's key: USERPROFILE (the ProfileList path expanded once with the system
// names only), and APPDATA and LOCALAPPDATA from the hive's User Shell Folders
// key, expanded once with USERPROFILE and the system names, else under
// AppData. It is empty when the profile path does not expand to a drive path.
func profileValues(registry Registry, securityID string,
	names map[string]string) map[string]string {
	values := map[string]string{}
	entry := registry.Strings(localMachine, profileListKey+securityID)
	raw, ok := lookupFold(entry.Text, "ProfileImagePath")
	if !ok {
		return values
	}
	known := newEnvironment(names)
	profile := strings.TrimRight(expandReferences(raw, known), `\`)
	if !isDrivePath(profile, `\`) || strings.Contains(profile, "%") {
		return values
	}
	values["USERPROFILE"] = profile
	values["APPDATA"] = profile + `\AppData\Roaming`
	values["LOCALAPPDATA"] = profile + `\AppData\Local`
	folders := registry.Strings(usersRoot, securityID+`\`+shellFoldersKey)
	known["userprofile"] = profile
	if roaming, ok := lookupFold(folders.Text, "AppData"); ok {
		values["APPDATA"] = expandReferences(roaming, known)
	}
	if local, ok := lookupFold(folders.Text, "Local AppData"); ok {
		values["LOCALAPPDATA"] = expandReferences(local, known)
	}
	return values
}

// acceptRegistryValue keeps a drive path (X:\ or X:/) holding no control
// character, %, quote or .. segment, less its trailing separators (a bare
// drive keeps one `\`). Otherwise value is empty and reason names the first
// problem in that order, naming as %NAME% the text between the first `%` and
// the next, which Windows would have read as a name. A `$` or a backquote is
// an ordinary Windows path character the registry does not expand.
func acceptRegistryValue(text string) (value, reason string) {
	_, afterPercent, percent := strings.Cut(text, "%")
	switch {
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
		if len(securityID) >= len(prefix) && equalFoldASCII(securityID[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}

// equalFoldASCII reports whether first and second are equal with ASCII
// letters compared without case, the fold used for registry names, security
// identifiers and Windows paths.
func equalFoldASCII(first, second string) bool {
	return len(first) == len(second) && lowerASCII(first) == lowerASCII(second)
}
