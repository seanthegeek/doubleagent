package users

import (
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/seanthegeek/doubleagent/internal/regtext"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// profileListKey holds one subkey per profile, named by the user's security
// identifier.
const profileListKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`

// profileReader lists the ProfileList entries in registry order.
type profileReader interface {
	Profiles() []Profile
}

// registryProfiles reads the machine's ProfileList.
type registryProfiles struct{}

// Profiles returns every ProfileList entry that has a ProfileImagePath, with
// the path's variables expanded whatever the value's type, in canonical form
// without a trailing separator. A key that cannot be read gives no entries,
// as does a subkey without a usable path.
func (registryProfiles) Profiles() []Profile {
	list, err := registry.OpenKey(registry.LOCAL_MACHINE, profileListKey,
		registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer list.Close()
	names, err := list.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	var profiles []Profile
	for _, name := range names {
		if path := profileImagePath(list, name); path != "" {
			profiles = append(profiles, Profile{SecurityID: name, User: leaf(path), Path: path})
		}
	}
	return profiles
}

// profileImagePath returns the expanded, canonical ProfileImagePath of the
// subkey, or "" when it has none.
func profileImagePath(list registry.Key, name string) string {
	key, err := registry.OpenKey(list, name, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := regtext.Read(key, "ProfileImagePath")
	if err != nil {
		return ""
	}
	// The PowerShell collector expands the path with
	// Environment.ExpandEnvironmentVariables, which hands it to the Win32
	// ExpandEnvironmentStrings as a NUL-terminated string, so a home is the
	// text before the first embedded NUL.
	value, _, _ = strings.Cut(value, "\x00")
	if value == "" {
		return ""
	}
	if expanded, err := registry.ExpandString(value); err == nil {
		value = expanded
	}
	return canonicalProfilePath(value)
}

// leaf returns the text after the last slash of a canonical path.
func leaf(path string) string {
	return path[strings.LastIndexByte(path, '/')+1:]
}

// canonicalProfilePath converts a registry path to canonical form and drops
// every trailing separator, as the PowerShell collector's TrimEnd does.
func canonicalProfilePath(value string) string {
	return strings.TrimRight(syspath.FromNative(value), "/")
}
