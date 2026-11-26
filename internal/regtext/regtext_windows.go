package regtext

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// Read returns the text of the value name under key, decoded as decode does,
// and the value's type. A missing value gives registry.ErrNotExist; a value
// that is neither REG_SZ nor REG_EXPAND_SZ gives its type and
// registry.ErrUnexpectedType without its data being read. A REG_EXPAND_SZ
// value is never expanded.
func Read(key registry.Key, name string) (text string, kind uint32, err error) {
	// The first call passes no buffer, so the registry reports only the type
	// and size; each later one passes a buffer of the last size reported,
	// again when the value grew in between.
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
		return decode(data[:n]), valueKind, nil
	}
}
