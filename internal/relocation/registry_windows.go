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

// Strings reads the values of subkey under root: a missing key gives nothing
// and no reason, a key that cannot be opened or listed gives the error's text.
// A value's text keeps its embedded NULs for acceptRegistryValue to refuse and
// is never expanded; a value of another type keeps only its name; the default
// (empty) name is skipped. ReadValueNames cuts a name at its first NUL, so a
// name may be listed twice (record keeps one hint per source) or read as
// missing: it is then marked Unreadable and the rest of the key is still
// read, so a value a user plants in their own key hides no other.
func (systemRegistry) Strings(root RegistryRoot, subkey string) RegistryStrings {
	values := RegistryStrings{Text: map[string]string{}, Expandable: map[string]bool{},
		Unreadable: map[string]bool{}}
	key, err := registry.OpenKey(rootKey(root), subkey, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		return values
	}
	if err != nil {
		values.Reason = err.Error()
		return values
	}
	defer key.Close()
	names, err := key.ReadValueNames(-1)
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
		case errors.Is(err, registry.ErrNotExist):
			values.Unreadable[name] = true
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
	if root == usersRoot {
		return registry.USERS
	}
	return registry.LOCAL_MACHINE
}
