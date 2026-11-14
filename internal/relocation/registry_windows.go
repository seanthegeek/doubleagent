package relocation

import (
	"errors"

	"golang.org/x/sys/windows/registry"

	"github.com/seanthegeek/doubleagent/internal/regtext"
)

// systemRegistry is the Registry of the running Windows host.
type systemRegistry struct{}

// SystemRegistry returns the Registry that reads this host's registry.
func SystemRegistry() Registry { return systemRegistry{} }

// Strings reads the values of subkey under root as Read-RegistryStrings
// does: a missing key gives nothing and no reason, a key that cannot be
// opened or listed gives the error's text as the reason. A value's text
// keeps its embedded NULs, as .NET's GetValue keeps them, so
// acceptRegistryValue refuses it rather than accepting the text before the
// first NUL. A REG_EXPAND_SZ value is never expanded, as
// DoNotExpandEnvironmentNames asks; a value of another type keeps only its
// name, and an empty name (the key's default value) is skipped. A name keeps
// a NUL it holds, as GetValueNames keeps it, so it matches no variable; its
// data is read by the name up to the NUL, as GetValue reads it.
func (systemRegistry) Strings(root RegistryRoot, subkey string) RegistryStrings {
	values := RegistryStrings{Text: map[string]string{}, Expandable: map[string]bool{}}
	key, err := registry.OpenKey(rootKey(root), subkey, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		return values
	}
	if err != nil {
		values.Reason = err.Error()
		return values
	}
	defer key.Close()
	names, err := regtext.ValueNames(key)
	if err != nil {
		values.Reason = err.Error()
		return values
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		values.Names = append(values.Names, name)
		text, kind, err := regtext.Read(key, name)
		switch {
		case errors.Is(err, registry.ErrUnexpectedType):
			continue
		case err != nil:
			values.Reason = err.Error()
			return values
		}
		values.Text[name] = text
		if kind == registry.EXPAND_SZ {
			values.Expandable[name] = true
		}
	}
	return values
}

// LoadedHives returns the key names under HKEY_USERS, nil when they cannot
// be listed.
func (systemRegistry) LoadedHives() []string {
	names, err := registry.USERS.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	return names
}

// rootKey returns the predefined key of root.
func rootKey(root RegistryRoot) registry.Key {
	if root == UsersRoot {
		return registry.USERS
	}
	return registry.LOCAL_MACHINE
}
