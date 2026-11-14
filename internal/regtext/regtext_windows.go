package regtext

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ValueNames returns the names of the values under key in registry order,
// each the whole counted name RegEnumValueW reports, decoded as Name does, as
// .NET's GetValueNames keeps it. registry.Key.ReadValueNames ends each name
// at its first NUL instead, so a value named CODEX_HOME<NUL>x would be listed
// as a second CODEX_HOME.
func ValueNames(key registry.Key) ([]string, error) {
	enumValue := windows.NewLazySystemDLL("advapi32.dll").NewProc("RegEnumValueW")
	if err := enumValue.Find(); err != nil {
		return nil, err
	}
	stat, err := key.Stat()
	if err != nil {
		return nil, err
	}
	units := make([]uint16, stat.MaxValueNameLen+1)
	var names []string
	for index := uint32(0); ; {
		size := uint32(len(units))
		status, _, _ := enumValue.Call(uintptr(key), uintptr(index),
			uintptr(unsafe.Pointer(&units[0])), uintptr(unsafe.Pointer(&size)), 0, 0, 0, 0)
		switch err := syscall.Errno(status); err {
		case 0:
			names = append(names, Name(units[:size]))
			index++
		case windows.ERROR_MORE_DATA:
			// A value added since the key's size was read has a longer name.
			units = make([]uint16, 2*len(units))
		case windows.ERROR_NO_MORE_ITEMS:
			return names, nil
		default:
			return names, err
		}
	}
}

// Read returns the text of the value name under key, decoded as Decode does,
// and the value's type. name is read up to its first NUL, as .NET's
// GetValue and GetValueKind pass it: their marshalling hands
// RegQueryValueExW a pointer to the string's characters, which the API reads
// as a NUL-terminated name, so CODEX_HOME<NUL>x reads CODEX_HOME. A missing value gives registry.ErrNotExist; a value
// that is neither REG_SZ nor REG_EXPAND_SZ gives its type and
// registry.ErrUnexpectedType without its data being read. A REG_EXPAND_SZ
// value is never expanded.
func Read(key registry.Key, name string) (text string, kind uint32, err error) {
	// The first call passes no buffer, so the registry reports only the type
	// and size; each later one passes a buffer of the last size reported,
	// again when the value grew in between.
	name, _, _ = strings.Cut(name, "\x00")
	var data []byte
	for {
		n, valueKind, err := key.GetValue(name, data)
		if err != nil && !errors.Is(err, syscall.ERROR_MORE_DATA) {
			return "", valueKind, err
		}
		if valueKind != registry.SZ && valueKind != registry.EXPAND_SZ {
			return "", valueKind, registry.ErrUnexpectedType
		}
		if n > len(data) {
			data = make([]byte, n)
			continue
		}
		if err != nil {
			return "", valueKind, err
		}
		return Decode(data[:n]), valueKind, nil
	}
}
